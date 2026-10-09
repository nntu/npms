package api

import (
	"context"
	"net/http"
	"sort"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"npms/backend/internal/api/contract"
)

func (s *Server) profileHandler() http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("NPMS API", "1.0.0"))
	huma.Get(api, "/api/v1/snmp/profiles", func(context.Context, *struct{}) (*contract.ProfileListOutput, error) {
		output := &contract.ProfileListOutput{}
		output.Body.Data = make([]contract.Profile, 0, len(s.profiles))
		for _, item := range s.profiles {
			keys := make([]string, 0, len(item.Counters))
			for key := range item.Counters {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			output.Body.Data = append(output.Body.Data, contract.Profile{
				ID: item.ID, Version: item.Version, Manufacturer: item.Manufacturer,
				VerificationStatus: string(item.VerificationStatus), CounterKeys: keys,
			})
		}
		output.Body.Total = len(output.Body.Data)
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
