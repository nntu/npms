package contract

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type Printer struct {
	ID           string  `json:"id" format:"uuid"`
	AssetCode    *string `json:"asset_code,omitempty"`
	DisplayName  string  `json:"display_name"`
	Manufacturer *string `json:"manufacturer,omitempty"`
	Model        *string `json:"model,omitempty"`
	Serial       *string `json:"serial,omitempty"`
	Department   *string `json:"department,omitempty"`
	Status       string  `json:"status" enum:"online,offline,unknown,unavailable"`
	LastSeenAt   *string `json:"last_seen_at,omitempty" format:"date-time"`
}

type PrinterPageOutput struct {
	Body struct {
		Data   []Printer `json:"data"`
		Limit  int       `json:"limit"`
		Offset int       `json:"offset"`
		Total  int       `json:"total"`
	}
}

type PrinterListInput struct {
	Limit  int `query:"limit,omitempty" minimum:"1" maximum:"1000" default:"50"`
	Offset int `query:"offset,omitempty" minimum:"0" default:"0"`
}

type PrinterIDInput struct {
	ID string `path:"id" format:"uuid"`
}

type CreatePrinterInput struct {
	Body struct {
		AssetCode    string `json:"asset_code,omitempty" maxLength:"200"`
		DisplayName  string `json:"display_name" minLength:"1" maxLength:"200"`
		Manufacturer string `json:"manufacturer,omitempty" maxLength:"200"`
		Model        string `json:"model,omitempty" maxLength:"200"`
		Serial       string `json:"serial,omitempty" maxLength:"200"`
		SysObjectID  string `json:"sys_object_id,omitempty" maxLength:"200"`
		Department   string `json:"department,omitempty" maxLength:"200"`
	}
}

type CreatePrinterOutput struct{ Body Printer }

type RegisterPrinterInput struct {
	Body struct {
		DisplayName    string `json:"display_name" minLength:"1" maxLength:"200"`
		AssetCode      string `json:"asset_code,omitempty" maxLength:"200"`
		Manufacturer   string `json:"manufacturer,omitempty" maxLength:"200"`
		Model          string `json:"model,omitempty" maxLength:"200"`
		Serial         string `json:"serial,omitempty" maxLength:"200"`
		SysObjectID    string `json:"sys_object_id,omitempty" maxLength:"200"`
		Department     string `json:"department,omitempty" maxLength:"200"`
		ProfileID      string `json:"profile_id,omitempty"`
		Address        string `json:"address" minLength:"1" maxLength:"253"`
		Port           int    `json:"port,omitempty" minimum:"1" maximum:"65535" default:"161"`
		Version        string `json:"version" enum:"2c,3"`
		Community      string `json:"community,omitempty" writeOnly:"true"`
		Username       string `json:"username,omitempty" writeOnly:"true"`
		AuthProtocol   string `json:"auth_protocol,omitempty" enum:"MD5,SHA"`
		AuthPassphrase string `json:"auth_passphrase,omitempty" writeOnly:"true"`
		PrivProtocol   string `json:"priv_protocol,omitempty" enum:"DES,AES,AES128"`
		PrivPassphrase string `json:"priv_passphrase,omitempty" writeOnly:"true"`
	}
}

type RegistrationOutput struct {
	Body struct {
		Printer                Printer `json:"printer"`
		CredentialID           string  `json:"credential_id"`
		EndpointID             string  `json:"endpoint_id"`
		PollJobID              *string `json:"poll_job_id,omitempty"`
		PollStatus             string  `json:"poll_status" enum:"queued,not_started"`
		ProfileID              *string `json:"profile_id,omitempty"`
		CounterDefinitionCount int     `json:"counter_definition_count" minimum:"0"`
	}
}

type CredentialInput struct {
	PrinterID string `path:"id" format:"uuid"`
	Body      struct {
		Version        string `json:"version" enum:"2c,3"`
		Community      string `json:"community,omitempty" writeOnly:"true"`
		Username       string `json:"username,omitempty" writeOnly:"true"`
		AuthProtocol   string `json:"auth_protocol,omitempty" enum:"MD5,SHA"`
		AuthPassphrase string `json:"auth_passphrase,omitempty" writeOnly:"true"`
		PrivProtocol   string `json:"priv_protocol,omitempty" enum:"DES,AES,AES128"`
		PrivPassphrase string `json:"priv_passphrase,omitempty" writeOnly:"true"`
	}
}

type CredentialOutput struct {
	Body struct {
		ID      string `json:"id"`
		Version string `json:"version" enum:"2c,3"`
	}
}

type EndpointInput struct {
	PrinterID string `path:"id" format:"uuid"`
	Body      struct {
		Address      string `json:"address" minLength:"1" maxLength:"253"`
		Protocol     string `json:"protocol,omitempty" enum:"snmp" default:"snmp"`
		Port         int    `json:"port,omitempty" minimum:"1" maximum:"65535" default:"161"`
		CredentialID string `json:"credential_id,omitempty"`
		IsPrimary    bool   `json:"is_primary,omitempty" default:"false"`
	}
}

type EndpointOutput struct {
	Body struct {
		ID           string  `json:"id"`
		DeviceID     string  `json:"device_id" format:"uuid"`
		Address      string  `json:"address"`
		Protocol     string  `json:"protocol" const:"snmp"`
		Port         int     `json:"port"`
		CredentialID *string `json:"credential_id,omitempty"`
		IsPrimary    bool    `json:"is_primary"`
	}
}

type Counter struct {
	ID            int64  `json:"id"`
	DefinitionKey string `json:"definition_key"`
	Unit          string `json:"unit" enum:"impressions,sheets"`
	Scope         string `json:"scope"`
	RawValue      int64  `json:"raw_value" minimum:"0"`
	CollectedAt   string `json:"collected_at" format:"date-time"`
	Quality       string `json:"quality" enum:"valid,unverified,unsupported,unavailable,suspicious"`
}

type CounterListInput struct {
	PrinterID string `path:"id" format:"uuid"`
	Limit     int    `query:"limit,omitempty" minimum:"1" maximum:"1000" default:"50"`
	Offset    int    `query:"offset,omitempty" minimum:"0" default:"0"`
}

type CounterPageOutput struct {
	Body struct {
		Data   []Counter `json:"data"`
		Limit  int       `json:"limit"`
		Offset int       `json:"offset"`
		Total  int       `json:"total"`
	}
}

type DailyUsage struct {
	DefinitionKey string `json:"definition_key"`
	Unit          string `json:"unit" enum:"impressions,sheets"`
	Scope         string `json:"scope"`
	LocalDate     string `json:"local_date" format:"date"`
	Delta         int64  `json:"delta" minimum:"0"`
	Quality       string `json:"quality" enum:"valid,unverified"`
}

type UsageInput struct {
	PrinterID string `path:"id" format:"uuid"`
	From      string `query:"from,omitempty" format:"date"`
	To        string `query:"to,omitempty" format:"date"`
	Timezone  string `query:"timezone,omitempty" default:"UTC"`
}

type UsagePageOutput struct {
	Body struct {
		Data     []DailyUsage `json:"data"`
		Total    int          `json:"total" minimum:"0"`
		Timezone string       `json:"timezone"`
	}
}

type PollingRun struct {
	ID             string  `json:"id"`
	JobKind        string  `json:"job_kind"`
	StartedAt      string  `json:"started_at" format:"date-time"`
	EndedAt        *string `json:"ended_at,omitempty" format:"date-time"`
	Result         string  `json:"result" enum:"running,success,failed"`
	ErrorCode      *string `json:"error_code,omitempty"`
	ProfileVersion int     `json:"profile_version,omitempty"`
	AttemptCount   int     `json:"attempt_count" minimum:"1"`
}

type PollingRunInput struct {
	PrinterID string `path:"id" format:"uuid"`
	Limit     int    `query:"limit,omitempty" minimum:"1" maximum:"1000" default:"50"`
	Offset    int    `query:"offset,omitempty" minimum:"0" default:"0"`
}

type PollingRunPageOutput struct {
	Body struct {
		Data   []PollingRun `json:"data"`
		Limit  int          `json:"limit"`
		Offset int          `json:"offset"`
		Total  int          `json:"total"`
	}
}

type JobAcceptedOutput struct {
	Body struct {
		JobID     string `json:"job_id"`
		Status    string `json:"status" const:"queued"`
		StatusURL string `json:"status_url"`
	}
}

type JobOutput struct {
	Body struct {
		ID        string  `json:"id"`
		DeviceID  string  `json:"device_id" format:"uuid"`
		Status    string  `json:"status" enum:"queued,success,failed"`
		ErrorCode *string `json:"error_code,omitempty"`
	}
}

type JobIDInput struct {
	ID string `path:"id"`
}

func registerPrinterContracts(api huma.API) {
	huma.Post(api, "/api/v1/printers/register", func(context.Context, *RegisterPrinterInput) (*RegistrationOutput, error) {
		return &RegistrationOutput{}, nil
	}, func(op *huma.Operation) { op.DefaultStatus = http.StatusCreated })
	huma.Post(api, "/api/v1/printers", func(context.Context, *CreatePrinterInput) (*CreatePrinterOutput, error) {
		return &CreatePrinterOutput{}, nil
	}, func(op *huma.Operation) { op.DefaultStatus = http.StatusCreated })
	huma.Get(api, "/api/v1/printers", func(context.Context, *PrinterListInput) (*PrinterPageOutput, error) { return &PrinterPageOutput{}, nil })
	huma.Get(api, "/api/v1/printers/{id}", func(context.Context, *PrinterIDInput) (*CreatePrinterOutput, error) {
		return &CreatePrinterOutput{}, nil
	})
	huma.Get(api, "/api/v1/printers/{id}/counters", func(context.Context, *CounterListInput) (*CounterPageOutput, error) { return &CounterPageOutput{}, nil })
	huma.Get(api, "/api/v1/printers/{id}/usage", func(context.Context, *UsageInput) (*UsagePageOutput, error) { return &UsagePageOutput{}, nil })
	huma.Post(api, "/api/v1/printers/{id}/credentials", func(context.Context, *CredentialInput) (*CredentialOutput, error) { return &CredentialOutput{}, nil }, func(op *huma.Operation) { op.DefaultStatus = http.StatusCreated })
	huma.Post(api, "/api/v1/printers/{id}/endpoints", func(context.Context, *EndpointInput) (*EndpointOutput, error) { return &EndpointOutput{}, nil }, func(op *huma.Operation) { op.DefaultStatus = http.StatusCreated })
	huma.Get(api, "/api/v1/printers/{id}/polling-runs", func(context.Context, *PollingRunInput) (*PollingRunPageOutput, error) {
		return &PollingRunPageOutput{}, nil
	})
	huma.Post(api, "/api/v1/printers/{id}/poll", func(context.Context, *PrinterIDInput) (*JobAcceptedOutput, error) { return &JobAcceptedOutput{}, nil }, func(op *huma.Operation) { op.DefaultStatus = http.StatusAccepted })
	huma.Get(api, "/api/v1/jobs/{id}", func(context.Context, *JobIDInput) (*JobOutput, error) { return &JobOutput{}, nil })
}
