package contract

import (
	"net/http"
	"testing"
)

func TestCartridgeContractRegistersAllOperations(t *testing.T) {
	_, spec := NewCartridgeAPI()
	want := map[string]string{
		"/api/v1/health":                          "get",
		"/api/v1/snmp/profiles":                   "get",
		"/api/v1/discovery/probe":                 "post",
		"/api/v1/cartridges":                      "get",
		"/api/v1/cartridges/stock":                "post",
		"/api/v1/cartridges/stock/refill-bottles": "post",
		"/api/v1/cartridges/replace":              "post",
		"/api/v1/cartridges/refill":               "post",
		"/api/v1/cartridges/refill-printer":       "post",
		"/api/v1/cartridges/logs":                 "get",
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
}
