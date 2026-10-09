// Package contract contains typed API contracts. The cartridge contract is
// currently shadow-registered so its generated OpenAPI can be compared with
// the production handlers before the router is migrated.
package contract

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

type Cartridge struct {
	ID                 string `json:"id"`
	SKUCode            string `json:"sku_code"`
	Name               string `json:"name"`
	CompatibleModels   string `json:"compatible_models,omitempty"`
	StockNew           int    `json:"stock_new" minimum:"0"`
	StockRefilled      int    `json:"stock_refilled" minimum:"0"`
	StockEmpty         int    `json:"stock_empty" minimum:"0"`
	StockRefillBottles int    `json:"stock_refill_bottles" minimum:"0"`
}

type CartridgeListInput struct{}
type CartridgeListOutput struct {
	Body struct {
		Data  []Cartridge `json:"data"`
		Total int         `json:"total"`
	}
}

type CreateCartridgeInput struct {
	Body struct {
		SKUCode            string `json:"sku_code" minLength:"1"`
		Name               string `json:"name" minLength:"1"`
		CompatibleModels   string `json:"compatible_models,omitempty"`
		StockNew           int    `json:"stock_new,omitempty" minimum:"0"`
		StockRefilled      int    `json:"stock_refilled,omitempty" minimum:"0"`
		StockEmpty         int    `json:"stock_empty,omitempty" minimum:"0"`
		StockRefillBottles int    `json:"stock_refill_bottles,omitempty" minimum:"0"`
	}
}
type CreateCartridgeOutput struct{ Body Cartridge }

type UpdateStockInput struct {
	Body struct {
		CartridgeID      string `json:"cartridge_id" minLength:"1"`
		AddStockNew      int    `json:"add_stock_new,omitempty"`
		AddStockRefilled int    `json:"add_stock_refilled,omitempty"`
		AddStockEmpty    int    `json:"add_stock_empty,omitempty"`
		Notes            string `json:"notes,omitempty"`
	}
}
type StatusOutput struct {
	Body struct {
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
	}
}

type AddRefillBottlesInput struct {
	Body struct {
		CartridgeID string `json:"cartridge_id" minLength:"1"`
		Quantity    int    `json:"quantity" minimum:"1"`
		Notes       string `json:"notes,omitempty"`
	}
}

type ReplaceCartridgeInput struct {
	Body struct {
		CartridgeID string `json:"cartridge_id" minLength:"1"`
		DeviceID    string `json:"device_id" minLength:"1"`
		SourceType  string `json:"source_type" enum:"new,refilled"`
		PageCount   int    `json:"page_count,omitempty" minimum:"1"`
		Notes       string `json:"notes,omitempty"`
	}
}
type ActionOutput struct {
	Body struct {
		Status         string `json:"status"`
		Message        string `json:"message"`
		Counter        int64  `json:"counter"`
		CounterQuality string `json:"counter_quality" enum:"valid,unverified,unsupported,unavailable,suspicious"`
	}
}

type RefillCartridgesInput struct {
	Body struct {
		CartridgeID string `json:"cartridge_id" minLength:"1"`
		Quantity    int    `json:"quantity" minimum:"1"`
		Notes       string `json:"notes,omitempty"`
	}
}

type RefillPrinterInput struct {
	Body struct {
		CartridgeID string `json:"cartridge_id" minLength:"1"`
		DeviceID    string `json:"device_id" minLength:"1"`
		Quantity    int    `json:"quantity" minimum:"1"`
		PageCount   int    `json:"page_count,omitempty" minimum:"1"`
		Notes       string `json:"notes,omitempty"`
	}
}

type LogsInput struct {
	CartridgeID string `query:"cartridge_id,omitempty"`
	DeviceID    string `query:"device_id,omitempty"`
	Limit       int    `query:"limit,omitempty" minimum:"1" maximum:"1000" default:"50"`
	Offset      int    `query:"offset,omitempty" minimum:"0" default:"0"`
}
type CartridgeLog struct {
	ID                 string `json:"id"`
	CartridgeID        string `json:"cartridge_id"`
	DeviceID           string `json:"device_id,omitempty"`
	ActionType         string `json:"action_type"`
	SourceType         string `json:"source_type,omitempty"`
	Quantity           int    `json:"quantity"`
	PageCount          int    `json:"page_count,omitempty"`
	CounterQuality     string `json:"counter_quality,omitempty"`
	CounterCollectedAt string `json:"counter_collected_at,omitempty"`
	Notes              string `json:"notes,omitempty"`
	PerformedAt        string `json:"performed_at"`
}
type LogsOutput struct {
	Body struct {
		Data   []CartridgeLog `json:"data"`
		Limit  int            `json:"limit"`
		Offset int            `json:"offset"`
		Total  int            `json:"total"`
	}
}

type HealthOutput struct {
	Body struct {
		Status string `json:"status" const:"ok"`
		Time   string `json:"time" format:"date-time"`
	}
}

type Profile struct {
	ID                 string   `json:"id"`
	Version            int      `json:"version" minimum:"1"`
	Manufacturer       string   `json:"manufacturer"`
	VerificationStatus string   `json:"verification_status" enum:"verified,unverified,experimental"`
	CounterKeys        []string `json:"counter_keys"`
}
type ProfileListOutput struct {
	Body struct {
		Data  []Profile `json:"data"`
		Total int       `json:"total"`
	}
}

type DiscoveryInput struct {
	Body struct {
		Address        string `json:"address" format:"ipv4" minLength:"1"`
		Port           int    `json:"port,omitempty" minimum:"1" maximum:"65535" default:"161"`
		Version        string `json:"version" enum:"2c,3"`
		Community      string `json:"community,omitempty" writeOnly:"true"`
		Username       string `json:"username,omitempty" writeOnly:"true"`
		AuthProtocol   string `json:"auth_protocol,omitempty" enum:"MD5,SHA"`
		AuthPassphrase string `json:"auth_passphrase,omitempty" writeOnly:"true"`
		PrivProtocol   string `json:"priv_protocol,omitempty" enum:"DES,AES,AES128"`
		PrivPassphrase string `json:"priv_passphrase,omitempty" writeOnly:"true"`
		Timeout        int    `json:"timeout,omitempty" minimum:"1" maximum:"30" default:"3"`
	}
}
type DiscoveryOutput struct {
	Body struct {
		Address     string `json:"address" format:"ipv4"`
		Name        string `json:"name"`
		Description string `json:"description"`
		SysObjectID string `json:"sys_object_id"`
		Serial      string `json:"serial"`
	}
}

func NewCartridgeAPI() (http.Handler, *huma.OpenAPI) {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("NPMS API", "1.0.0"))
	noop := func(context.Context) (*StatusOutput, error) { return &StatusOutput{}, nil }
	huma.Get(api, "/api/v1/health", func(context.Context, *struct{}) (*HealthOutput, error) { return &HealthOutput{}, nil })
	huma.Get(api, "/api/v1/snmp/profiles", func(context.Context, *struct{}) (*ProfileListOutput, error) { return &ProfileListOutput{}, nil })
	huma.Post(api, "/api/v1/discovery/probe", func(context.Context, *DiscoveryInput) (*DiscoveryOutput, error) { return &DiscoveryOutput{}, nil })
	registerPrinterContracts(api)

	huma.Get(api, "/api/v1/cartridges", func(context.Context, *CartridgeListInput) (*CartridgeListOutput, error) {
		return &CartridgeListOutput{}, nil
	})
	huma.Post(api, "/api/v1/cartridges", func(context.Context, *CreateCartridgeInput) (*CreateCartridgeOutput, error) {
		return &CreateCartridgeOutput{}, nil
	}, func(op *huma.Operation) { op.DefaultStatus = http.StatusCreated })
	huma.Post(api, "/api/v1/cartridges/stock", func(ctx context.Context, input *UpdateStockInput) (*StatusOutput, error) { return noop(ctx) })
	huma.Post(api, "/api/v1/cartridges/stock/refill-bottles", func(ctx context.Context, input *AddRefillBottlesInput) (*StatusOutput, error) { return noop(ctx) })
	huma.Post(api, "/api/v1/cartridges/replace", func(ctx context.Context, input *ReplaceCartridgeInput) (*ActionOutput, error) {
		return &ActionOutput{}, nil
	})
	huma.Post(api, "/api/v1/cartridges/refill", func(ctx context.Context, input *RefillCartridgesInput) (*StatusOutput, error) { return noop(ctx) })
	huma.Post(api, "/api/v1/cartridges/refill-printer", func(ctx context.Context, input *RefillPrinterInput) (*ActionOutput, error) {
		return &ActionOutput{}, nil
	})
	huma.Get(api, "/api/v1/cartridges/logs", func(context.Context, *LogsInput) (*LogsOutput, error) { return &LogsOutput{}, nil })
	return mux, api.OpenAPI()
}
