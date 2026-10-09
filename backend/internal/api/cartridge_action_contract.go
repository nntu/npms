package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"npms/backend/internal/api/contract"
	"npms/backend/internal/counter"
	"npms/backend/internal/repository"
)

func (s *Server) cartridgeActionHandler() http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("NPMS API", "1.0.0"))

	huma.Post(api, "/api/v1/cartridges/replace", func(ctx context.Context, input *contract.ReplaceCartridgeInput) (*contract.ActionOutput, error) {
		store, ok := s.store.(CartridgeStore)
		if !ok {
			return nil, huma.Error503ServiceUnavailable("cartridge management is not configured")
		}
		params := repository.ReplaceCartridgeParams{LogID: newJobID(), CartridgeID: strings.TrimSpace(input.Body.CartridgeID), DeviceID: strings.TrimSpace(input.Body.DeviceID), SourceType: strings.TrimSpace(input.Body.SourceType), PageCount: input.Body.PageCount, Notes: strings.TrimSpace(input.Body.Notes)}
		counterStatus := "unavailable"
		if input.Body.PageCount > 0 {
			params.CounterQuality = string(counter.QualityUnverified)
			counterStatus = params.CounterQuality
		} else if s.counterSnapshotter != nil {
			reading, _, err := s.counterSnapshotter.ReadDevice(ctx, params.DeviceID)
			if err == nil {
				params.PageCount = int(reading.RawValue)
				params.CounterQuality = string(reading.Quality)
				params.CounterCollectedAt = reading.CollectedAt
				counterStatus = params.CounterQuality
			}
		}
		if err := store.ReplacePrinterCartridge(ctx, params); err != nil {
			return nil, huma.Error400BadRequest("cartridge replace failed", err)
		}
		return actionOutput("success", "cartridge replaced successfully", int64(params.PageCount), counterStatus), nil
	})
	huma.Post(api, "/api/v1/cartridges/refill", func(ctx context.Context, input *contract.RefillCartridgesInput) (*contract.StatusOutput, error) {
		store, ok := s.store.(CartridgeStore)
		if !ok {
			return nil, huma.Error503ServiceUnavailable("cartridge management is not configured")
		}
		if err := store.RefillCartridges(ctx, newJobID(), strings.TrimSpace(input.Body.CartridgeID), input.Body.Quantity, strings.TrimSpace(input.Body.Notes)); err != nil {
			return nil, huma.Error400BadRequest("cartridge refill failed", err)
		}
		return statusOutput("success", "cartridges refilled successfully"), nil
	})
	huma.Post(api, "/api/v1/cartridges/refill-printer", func(ctx context.Context, input *contract.RefillPrinterInput) (*contract.ActionOutput, error) {
		store, ok := s.store.(CartridgeStore)
		if !ok {
			return nil, huma.Error503ServiceUnavailable("cartridge management is not configured")
		}
		params := repository.RefillPrinterCartridgeParams{LogID: newJobID(), CartridgeID: strings.TrimSpace(input.Body.CartridgeID), DeviceID: strings.TrimSpace(input.Body.DeviceID), Quantity: input.Body.Quantity, PageCount: input.Body.PageCount, Notes: strings.TrimSpace(input.Body.Notes)}
		counterStatus := "unavailable"
		if input.Body.PageCount > 0 {
			params.CounterQuality = string(counter.QualityUnverified)
			counterStatus = params.CounterQuality
		} else if s.counterSnapshotter != nil {
			reading, _, err := s.counterSnapshotter.ReadDevice(ctx, params.DeviceID)
			if err == nil {
				params.PageCount = int(reading.RawValue)
				params.CounterQuality = string(reading.Quality)
				params.CounterCollectedAt = reading.CollectedAt
				counterStatus = params.CounterQuality
			}
		}
		if err := store.RefillPrinterCartridge(ctx, params); err != nil {
			return nil, huma.Error400BadRequest("printer refill failed", err)
		}
		return actionOutput("success", "printer cartridge refilled successfully", int64(params.PageCount), counterStatus), nil
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func actionOutput(status, message string, value int64, quality string) *contract.ActionOutput {
	output := &contract.ActionOutput{}
	output.Body.Status = status
	output.Body.Message = message
	output.Body.Counter = value
	output.Body.CounterQuality = quality
	return output
}

func statusOutput(status, message string) *contract.StatusOutput {
	output := &contract.StatusOutput{}
	output.Body.Status = status
	output.Body.Message = message
	return output
}
