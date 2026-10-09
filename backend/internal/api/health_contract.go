package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"npms/backend/internal/api/contract"
)

// healthHandler is the first production endpoint migrated to the typed Huma
// boundary. The remaining legacy endpoints continue to use the same outer
// middleware until their group is migrated and contract-tested.
func (s *Server) healthHandler() http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("NPMS API", "1.0.0"))
	huma.Get(api, "/api/v1/health", func(context.Context, *struct{}) (*contract.HealthOutput, error) {
		output := &contract.HealthOutput{}
		output.Body.Status = "ok"
		output.Body.Time = time.Now().UTC().Format(time.RFC3339Nano)
		return output, nil
	})
	return mux
}
