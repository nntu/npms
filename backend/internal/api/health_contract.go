package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"npms/backend/internal/api/contract"
	"npms/backend/internal/version"
)

// healthHandler is the first production endpoint migrated to the typed Huma
// boundary. The remaining legacy endpoints continue to use the same outer
// middleware until their group is migrated and contract-tested.
func (s *Server) healthHandler() http.Handler {
	mux := http.NewServeMux()
	info := version.Get()
	api := humago.New(mux, huma.DefaultConfig("NPMS API", info.Version))
	huma.Get(api, "/api/v1/health", func(context.Context, *struct{}) (*contract.HealthOutput, error) {
		return healthOutput(), nil
	})
	huma.Get(api, "/api/v1/health/live", func(context.Context, *struct{}) (*contract.HealthOutput, error) { return healthOutput(), nil })
	huma.Get(api, "/api/v1/health/ready", func(ctx context.Context, _ *struct{}) (*contract.HealthOutput, error) {
		if _, err := s.store.CountDevices(ctx); err != nil {
			return nil, huma.Error503ServiceUnavailable("database is not ready")
		}
		return healthOutput(), nil
	})
	huma.Get(api, "/api/v1/version", func(context.Context, *struct{}) (*contract.VersionOutput, error) {
		return versionOutput(), nil
	})
	return mux
}

func healthOutput() *contract.HealthOutput {
	info := version.Get()
	output := &contract.HealthOutput{}
	output.Body.Status = "ok"
	output.Body.Time = time.Now().UTC().Format(time.RFC3339Nano)
	output.Body.Version = info.Version
	output.Body.GitCommit = info.GitCommit
	output.Body.BuildDate = info.BuildDate
	return output
}

func versionOutput() *contract.VersionOutput {
	info := version.Get()
	output := &contract.VersionOutput{}
	output.Body.Version = info.Version
	output.Body.GitCommit = info.GitCommit
	output.Body.BuildDate = info.BuildDate
	return output
}
