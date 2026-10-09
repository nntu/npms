package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"npms/backend/internal/api/contract"
)

func (s *Server) cartridgeStockHandler() http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("NPMS API", "1.0.0"))
	huma.Post(api, "/api/v1/cartridges/stock", func(ctx context.Context, input *contract.UpdateStockInput) (*contract.StatusOutput, error) {
		store, ok := s.store.(CartridgeStore)
		if !ok {
			return nil, huma.Error503ServiceUnavailable("cartridge management is not configured")
		}
		if err := store.UpdateCartridgeStock(ctx, strings.TrimSpace(input.Body.CartridgeID), input.Body.AddStockNew, input.Body.AddStockRefilled, input.Body.AddStockEmpty, newJobID(), strings.TrimSpace(input.Body.Notes)); err != nil {
			return nil, huma.Error400BadRequest("could not update cartridge stock", err)
		}
		output := &contract.StatusOutput{}
		output.Body.Status = "ok"
		return output, nil
	})
	huma.Post(api, "/api/v1/cartridges/stock/refill-bottles", func(ctx context.Context, input *contract.AddRefillBottlesInput) (*contract.StatusOutput, error) {
		store, ok := s.store.(CartridgeStore)
		if !ok {
			return nil, huma.Error503ServiceUnavailable("cartridge management is not configured")
		}
		if err := store.AddRefillBottles(ctx, strings.TrimSpace(input.Body.CartridgeID), input.Body.Quantity, newJobID(), strings.TrimSpace(input.Body.Notes)); err != nil {
			return nil, huma.Error400BadRequest("could not add refill bottles", err)
		}
		output := &contract.StatusOutput{}
		output.Body.Status = "ok"
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
