package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"npms/backend/internal/api/contract"
)

func (s *Server) cartridgeListHandler() http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("NPMS API", "1.0.0"))
	huma.Get(api, "/api/v1/cartridges", func(ctx context.Context, _ *contract.CartridgeListInput) (*contract.CartridgeListOutput, error) {
		store, ok := s.store.(CartridgeStore)
		if !ok {
			return nil, huma.Error503ServiceUnavailable("cartridge management is not configured")
		}
		items, err := store.ListCartridges(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("could not list cartridges", err)
		}
		output := &contract.CartridgeListOutput{}
		output.Body.Data = make([]contract.Cartridge, 0, len(items))
		for _, item := range items {
			output.Body.Data = append(output.Body.Data, contract.Cartridge{
				ID: item.ID, SKUCode: item.SKUCode, Name: item.Name,
				CompatibleModels: item.CompatibleModels, StockNew: item.StockNew,
				StockRefilled: item.StockRefilled, StockEmpty: item.StockEmpty,
				StockRefillBottles: item.StockRefillBottles,
			})
		}
		output.Body.Total = len(output.Body.Data)
		return output, nil
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			s.handleCartridges(w, r)
			return
		}
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
			return
		}
		mux.ServeHTTP(w, r)
	})
}
