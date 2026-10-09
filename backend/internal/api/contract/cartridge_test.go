package contract

import (
	"net/http"
	"testing"
)

func TestCartridgeContractRegistersAllOperations(t *testing.T) {
	_, spec := NewCartridgeAPI()
	want := map[string]string{
		"/api/v1/health":                          "get",
		"/api/v1/health/live":                     "get",
		"/api/v1/health/ready":                    "get",
		"/api/v1/snmp/profiles":                   "get",
		"/api/v1/discovery/probe":                 "post",
		"/api/v1/cartridges":                      "get",
		"/api/v1/cartridges/stock":                "post",
		"/api/v1/cartridges/stock/refill-bottles": "post",
		"/api/v1/cartridges/replace":              "post",
		"/api/v1/cartridges/refill":               "post",
		"/api/v1/cartridges/refill-printer":       "post",
		"/api/v1/cartridges/logs":                 "get",
		"/api/v1/printers/register":               "post",
		"/api/v1/printers":                        "get",
		"/api/v1/printers/{id}":                   "get",
		"/api/v1/printers/{id}/counters":          "get",
		"/api/v1/printers/{id}/usage":             "get",
		"/api/v1/printers/{id}/credentials":       "post",
		"/api/v1/printers/{id}/endpoints":         "post",
		"/api/v1/printers/{id}/polling-runs":      "get",
		"/api/v1/printers/{id}/poll":              "post",
		"/api/v1/jobs/{id}":                       "get",
	}
	for path, method := range want {
		item, ok := spec.Paths[path]
		if !ok {
			t.Fatalf("missing contract path %s", path)
		}
		if method == http.MethodGet && item.Get == nil {
			t.Fatalf("missing GET contract for %s", path)
		}
		if method == http.MethodPost && item.Post == nil {
			t.Fatalf("missing POST contract for %s", path)
		}
	}
	if spec.Components == nil || spec.Components.SecuritySchemes["bearerAuth"] == nil {
		t.Fatal("missing bearer authentication scheme")
	}
	if spec.Paths["/api/v1/cartridges"].Get.Security == nil {
		t.Fatal("cartridge list must declare bearer authentication")
	}
	if spec.Paths["/api/v1/health"].Get.Security != nil {
		t.Fatal("health must remain public")
	}
}
