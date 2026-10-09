package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"npms/backend/internal/api/contract"
)

func (s *Server) cartridgeLogsHandler() http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("NPMS API", "1.0.0"))
	huma.Get(api, "/api/v1/cartridges/logs", func(ctx context.Context, input *contract.LogsInput) (*contract.LogsOutput, error) {
		store, ok := s.store.(CartridgeStore)
		if !ok {
			return nil, huma.Error503ServiceUnavailable("cartridge management is not configured")
		}
		limit, offset := input.Limit, input.Offset
		if limit == 0 {
			limit = 50
		}
		logs, err := store.ListCartridgeLogs(ctx, strings.TrimSpace(input.CartridgeID), strings.TrimSpace(input.DeviceID), limit, offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("could not list cartridge logs", err)
		}
		total, err := store.CountCartridgeLogs(ctx, strings.TrimSpace(input.CartridgeID), strings.TrimSpace(input.DeviceID))
		if err != nil {
			return nil, huma.Error500InternalServerError("could not count cartridge logs", err)
		}
		output := &contract.LogsOutput{}
		output.Body.Limit, output.Body.Offset, output.Body.Total = limit, offset, total
		output.Body.Data = make([]contract.CartridgeLog, 0, len(logs))
		for _, item := range logs {
			counterCollectedAt, performedAt := "", ""
			if !item.CounterCollectedAt.IsZero() {
				counterCollectedAt = item.CounterCollectedAt.Format("2006-01-02T15:04:05.999999999Z07:00")
			}
			if !item.PerformedAt.IsZero() {
				performedAt = item.PerformedAt.Format("2006-01-02T15:04:05.999999999Z07:00")
			}
			output.Body.Data = append(output.Body.Data, contract.CartridgeLog{
				ID: item.ID, CartridgeID: item.CartridgeID, DeviceID: item.DeviceID,
				ActionType: item.ActionType, SourceType: item.SourceType, Quantity: item.Quantity,
				PageCount: item.PageCount, PrintedPages: item.PrintedPages, CounterQuality: item.CounterQuality,
				CounterCollectedAt: counterCollectedAt,
				Notes:              item.Notes, PerformedAt: performedAt,
			})
		}
		return output, nil
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
			return
		}
		mux.ServeHTTP(w, r)
	})
}
