package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type fanoutHandler struct {
	console slog.Handler
	file    slog.Handler
}

type rotatingWriter struct {
	mu       sync.Mutex
	basePath string
	maxBytes int64
	daily    bool
	file     *os.File
	filePath string
	day      string
	part     int
	bytes    int64
}

func newRotatingWriter(basePath string, maxBytes int64, daily bool) (*rotatingWriter, error) {
	if maxBytes < 1 {
		return nil, fmt.Errorf("max log size must be positive")
	}
	writer := &rotatingWriter{basePath: basePath, maxBytes: maxBytes, daily: daily}
	if err := writer.rotateLocked(time.Now().UTC()); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *rotatingWriter) Write(payload []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now().UTC()
	dayChanged := w.daily && w.day != now.Format("2006-01-02")
	tooLarge := w.bytes > 0 && w.bytes+int64(len(payload)) > w.maxBytes
	if w.file == nil || dayChanged || tooLarge {
		if tooLarge && !dayChanged {
			w.part++
		}
		if err := w.rotateLocked(now); err != nil {
			return 0, err
		}
	}
	count, err := w.file.Write(payload)
	w.bytes += int64(count)
	return count, err
}

func (w *rotatingWriter) rotateLocked(now time.Time) error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
		w.file = nil
	}
	w.day = now.Format("2006-01-02")
	for {
		path := w.pathLocked()
		info, err := os.Stat(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Size() < w.maxBytes {
			w.bytes = info.Size()
			break
		}
		if err == nil {
			w.part++
			continue
		}
		w.bytes = 0
		break
	}
	if err := os.MkdirAll(filepath.Dir(w.pathLocked()), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(w.pathLocked(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	w.file = file
	return nil
}

func (w *rotatingWriter) pathLocked() string {
	if !w.daily {
		if w.part == 0 {
			return w.basePath
		}
		return addSuffix(w.basePath, fmt.Sprintf(".part-%03d", w.part))
	}
	datePath := addSuffix(w.basePath, "-"+w.day)
	if w.part > 0 {
		datePath = addSuffix(datePath, fmt.Sprintf(".part-%03d", w.part))
	}
	return datePath
}

func addSuffix(path, suffix string) string {
	extension := filepath.Ext(path)
	if extension == "" {
		return path + suffix
	}
	return strings.TrimSuffix(path, extension) + suffix + extension
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	if err := w.file.Sync(); err != nil {
		_ = w.file.Close()
		w.file = nil
		return err
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (h fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.console.Enabled(ctx, level) || h.file.Enabled(ctx, level)
}

func (h fanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.console.Enabled(ctx, record.Level) {
		if err := h.console.Handle(ctx, record); err != nil {
			return err
		}
	}
	if h.file.Enabled(ctx, record.Level) {
		if err := h.file.Handle(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (h fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return fanoutHandler{console: h.console.WithAttrs(attrs), file: h.file.WithAttrs(attrs)}
}

func (h fanoutHandler) WithGroup(name string) slog.Handler {
	return fanoutHandler{console: h.console.WithGroup(name), file: h.file.WithGroup(name)}
}

// Setup keeps human-readable logs on stderr and writes only error-level
// records as JSON lines to a permission-protected file.
func Setup(errorPath string, daily bool, maxSizeMB int) (func() error, error) {
	if errorPath == "" {
		errorPath = "./data/npms-errors.log"
	}
	if maxSizeMB < 1 {
		maxSizeMB = 10
	}
	writer, err := newRotatingWriter(errorPath, int64(maxSizeMB)*1024*1024, daily)
	if err != nil {
		return nil, err
	}
	console := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	errorFile := slog.NewJSONHandler(io.Writer(writer), &slog.HandlerOptions{Level: slog.LevelError})
	slog.SetDefault(slog.New(fanoutHandler{console: console, file: errorFile}))
	return func() error {
		return writer.Close()
	}, nil
}
