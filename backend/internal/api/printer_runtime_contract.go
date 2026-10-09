package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"npms/backend/internal/api/contract"
	"npms/backend/internal/repository"
)

func (s *Server) printerCollectionHandler() http.Handler {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("NPMS API", "1.0.0"))
	huma.Get(api, "/api/v1/printers", func(ctx context.Context, input *contract.PrinterListInput) (*contract.PrinterPageOutput, error) {
		limit, offset := input.Limit, input.Offset
		if limit == 0 {
			limit = 50
		}
		devices, err := s.store.ListDevices(ctx, limit, offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("could not list printers", err)
		}
		total, err := s.store.CountDevices(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("could not count printers", err)
		}
		output := &contract.PrinterPageOutput{}
		output.Body.Limit, output.Body.Offset, output.Body.Total = limit, offset, total
		output.Body.Data = make([]contract.Printer, 0, len(devices))
		for _, device := range devices {
			output.Body.Data = append(output.Body.Data, printerContract(device))
		}
		return output, nil
	})
	huma.Post(api, "/api/v1/printers", func(ctx context.Context, input *contract.CreatePrinterInput) (*contract.CreatePrinterOutput, error) {
		body := input.Body
		device := repository.Device{ID: newJobID(), AssetCode: strings.TrimSpace(body.AssetCode), DisplayName: strings.TrimSpace(body.DisplayName), Manufacturer: strings.TrimSpace(body.Manufacturer), Model: strings.TrimSpace(body.Model), Serial: strings.TrimSpace(body.Serial), SysObjectID: strings.TrimSpace(body.SysObjectID), Department: strings.TrimSpace(body.Department), Status: "unknown"}
		if err := s.store.CreateDevice(ctx, device); err != nil {
			return nil, huma.Error409Conflict("could not create printer", err)
		}
		output := &contract.CreatePrinterOutput{}
		output.Body = printerContract(device)
		return output, nil
	}, func(op *huma.Operation) { op.DefaultStatus = http.StatusCreated })

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
			return
		}
		if r.Method == http.MethodGet {
			if _, _, err := pagination(r); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_pagination", err.Error())
				return
			}
		}
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(io.LimitReader(r.Body, 32<<10+1))
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
				return
			}
			var fields map[string]json.RawMessage
			if err := json.NewDecoder(bytes.NewReader(body)).Decode(&fields); err == nil {
				allowed := map[string]bool{"asset_code": true, "display_name": true, "manufacturer": true, "model": true, "serial": true, "sys_object_id": true, "department": true}
				for key := range fields {
					if !allowed[key] {
						writeError(w, http.StatusBadRequest, "invalid_json", "request body contains unknown fields")
						return
					}
				}
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		mux.ServeHTTP(w, r)
	})
}

func printerContract(device repository.Device) contract.Printer {
	response := toPrinterResponse(device)
	return contract.Printer{ID: response.ID, AssetCode: response.AssetCode, DisplayName: response.DisplayName, Manufacturer: response.Manufacturer, Model: response.Model, Serial: response.Serial, Department: response.Department, Status: response.Status, LastSeenAt: response.LastSeenAt}
}
