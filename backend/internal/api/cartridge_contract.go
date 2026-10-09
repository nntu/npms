package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"npms/backend/internal/api/contract"
	"npms/backend/internal/repository"
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
	huma.Post(api, "/api/v1/cartridges", func(ctx context.Context, input *contract.CreateCartridgeInput) (*contract.CreateCartridgeOutput, error) {
		store, ok := s.store.(CartridgeStore)
		if !ok {
			return nil, huma.Error503ServiceUnavailable("cartridge management is not configured")
		}
		item := repository.Cartridge{
			ID: newJobID(), SKUCode: strings.TrimSpace(input.Body.SKUCode),
			Name: strings.TrimSpace(input.Body.Name), CompatibleModels: strings.TrimSpace(input.Body.CompatibleModels),
			StockNew: input.Body.StockNew, StockRefilled: input.Body.StockRefilled,
			StockEmpty: input.Body.StockEmpty, StockRefillBottles: input.Body.StockRefillBottles,
		}
		if err := store.CreateCartridge(ctx, item); err != nil {
			return nil, huma.Error400BadRequest("could not create cartridge", err)
		}
		output := &contract.CreateCartridgeOutput{}
		output.Body = contract.Cartridge{ID: item.ID, SKUCode: item.SKUCode, Name: item.Name, CompatibleModels: item.CompatibleModels, StockNew: item.StockNew, StockRefilled: item.StockRefilled, StockEmpty: item.StockEmpty, StockRefillBottles: item.StockRefillBottles}
		return output, nil
	}, func(op *huma.Operation) { op.DefaultStatus = http.StatusCreated })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
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
