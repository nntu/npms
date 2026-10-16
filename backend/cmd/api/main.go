package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"npms/backend/internal/api"
	"npms/backend/internal/config"
	"npms/backend/internal/database"
	"npms/backend/internal/discovery"
	"npms/backend/internal/ingestion"
	"npms/backend/internal/logging"
	"npms/backend/internal/polling"
	"npms/backend/internal/profile"
	"npms/backend/internal/repository"
	"npms/backend/internal/runtime"
	"npms/backend/internal/security"
	"npms/backend/internal/snmp"
	"npms/backend/internal/version"
	"npms/backend/web"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "init" || os.Args[1] == "-init" || os.Args[1] == "--init") {
		initCmd := flag.NewFlagSet("init", flag.ExitOnError)
		configPath := initCmd.String("config", "./config.yaml", "Target YAML configuration path")
		templatePath := initCmd.String("template", "./config.example.yaml", "Source template configuration path")
		force := initCmd.Bool("force", false, "Overwrite existing configuration file")
		genAPIToken := initCmd.Bool("generate-api-token", false, "Generate API token")
		customKey := initCmd.String("key", "", "Custom 32-byte encryption key")
		_ = initCmd.Parse(os.Args[2:])

		res, err := config.InitConfig(config.InitOptions{
			Path:             *configPath,
			TemplatePath:     *templatePath,
			Force:            *force,
			GenerateAPIToken: *genAPIToken,
			EncryptionKey:    *customKey,
		})
		if err != nil {
			fail(err)
		}
		fmt.Printf("NPMS configuration initialized at %s\nSecrets were written to the permission-protected configuration file.\n", res.ConfigPath)
		return
	}

	configPath := flag.String("config", "./config.yaml", "YAML configuration path")
	openBrowserFlag := flag.Bool("open-browser", false, "Force open web browser on startup")
	flag.BoolVar(openBrowserFlag, "b", false, "Force open web browser on startup (shorthand)")
	noBrowserFlag := flag.Bool("no-browser", false, "Disable automatic browser opening")
	showVersionFlag := flag.Bool("version", false, "Show version and build info")
	flag.BoolVar(showVersionFlag, "v", false, "Show version and build info (shorthand)")
	flag.Parse()

	if *showVersionFlag {
		info := version.Get()
		fmt.Printf("NPMS v%s (commit: %s, built: %s)\n", info.Version, info.GitCommit, info.BuildDate)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fail(err)
	}

	closeLog, err := logging.Setup(cfg.Logging.ErrorFile, cfg.Logging.Daily, cfg.Logging.MaxSizeMB)
	if err != nil {
		fail(fmt.Errorf("setup error log: %w", err))
	}
	defer func() { _ = closeLog() }()

	appInfo := version.Get()
	slog.Info("npms starting", "version", appInfo.Version, "commit", appInfo.GitCommit, "build_date", appInfo.BuildDate)
	profiles, err := profile.LoadDir(cfg.Profiles.Path)
	if err != nil {
		fail(err)
	}
	if warnings := profile.LintProfiles(profiles); len(warnings) > 0 {
		for _, w := range warnings {
			slog.Warn("profile lint warning", "warning", w)
		}
	}
	slog.Info("profiles loaded", "directory", cfg.Profiles.Path, "count", len(profiles))

	db, err := database.OpenSQLite(cfg.Database.Path)
	if err != nil {
		fail(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		fail(err)
	}
	store, err := repository.NewSQLiteRepository(db.DB)
	if err != nil {
		fail(err)
	}
	server, err := api.NewServer(store, "", cfg.Server.AllowedOrigin)
	if err != nil {
		fail(err)
	}
	server.SetFrontendFS(web.Assets())
	server.SetProfiles(profiles)
	server.SetDatabaseBackupManager(db)

	key, err := security.ParseKey(cfg.Security.EncryptionKey)
	if err != nil {
		fail(err)
	}
	box, err := security.NewSecretBox(key)
	if err != nil {
		fail(err)
	}
	server.SetSecretBox(box)
	server.SetDiscoveryProbe(func(ctx context.Context, config snmp.Config) (discovery.Result, error) {
		return discovery.Probe(ctx, config, ingestion.ConnectSNMP)
	})
	server.SetSubnetScanner(func(ctx context.Context, config discovery.SubnetScanConfig) ([]discovery.Result, error) {
		return discovery.ScanSubnet(ctx, config, ingestion.ConnectSNMP)
	})
	poller, err := ingestion.NewStatusPoller(store, ingestion.ConfigResolver{Store: store, Box: box}, ingestion.ConnectSNMP)
	if err != nil {
		fail(err)
	}
	counterPoller, err := ingestion.NewCounterPoller(ingestion.ConfigResolver{Store: store, Box: box}, ingestion.ConnectSNMP)
	if err != nil {
		fail(err)
	}
	server.SetCounterSnapshotter(counterPoller)
	profileTester, err := ingestion.NewProfileTester(ingestion.ConfigResolver{Store: store, Box: box}, ingestion.ConnectSNMP)
	if err != nil {
		fail(err)
	}
	server.SetProfileTester(profileTester)

	statusInterval, err := cfg.StatusInterval()
	if err != nil {
		fail(err)
	}
	counterInterval, err := cfg.CounterInterval()
	if err != nil {
		fail(err)
	}
	cleanupInterval, err := cfg.CleanupInterval()
	if err != nil {
		fail(err)
	}
	reportLocation, err := time.LoadLocation(cfg.Report.Timezone)
	if err != nil {
		fail(fmt.Errorf("load report timezone: %w", err))
	}
	scheduler, err := polling.NewScheduler(polling.Config{Concurrency: cfg.Polling.Concurrency, MaxAttempts: 2, RetryDelay: time.Second})
	if err != nil {
		fail(err)
	}
	counterService, err := polling.NewService(scheduler, store)
	if err != nil {
		fail(err)
	}
	now := time.Now().UTC()
	retentionPolicy := repository.CleanupPolicy{
		PollingRunsBefore:   now.AddDate(0, 0, -cfg.Retention.PollingRunsDays),
		CounterEventsBefore: now.AddDate(0, 0, -cfg.Retention.CounterEventsDays),
		JobsBefore:          now.AddDate(0, 0, -cfg.Retention.JobsDays),
	}
	runner, err := runtime.NewRunner(store, poller, counterPoller, scheduler, counterService, retentionPolicy, reportLocation)
	if err != nil {
		fail(err)
	}
	server.SetStatusPoller(runner)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("background polling worker started", "status_interval", statusInterval, "counter_interval", counterInterval, "cleanup_interval", cleanupInterval)
		if err := runtime.RunLoop(ctx, runner, statusInterval, counterInterval, cleanupInterval); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("background worker stopped", "error", err)
		}
	}()

	shouldOpenBrowser := (*openBrowserFlag || cfg.Server.OpenBrowser) && !*noBrowserFlag
	if shouldOpenBrowser {
		go func() {
			time.Sleep(200 * time.Millisecond)
			webURL := runtime.FormatServerURL(cfg.Server.Listen)
			slog.Info("opening web browser", "url", webURL)
			if err := runtime.OpenBrowser(webURL); err != nil {
				slog.Warn("failed to open browser", "error", err, "url", webURL)
			}
		}()
	}

	httpServer := &http.Server{Addr: cfg.Server.Listen, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	slog.Info("api server listening", "address", cfg.Server.Listen, "database", cfg.Database.Path)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fail(err)
	}

}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
