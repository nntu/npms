// Command api-contract exports the shadow typed contract for comparison during
// the Huma migration. It does not serve production traffic.
package main

import (
	"flag"
	"fmt"
	"os"

	"npms/backend/internal/api/contract"
)

func main() {
	output := flag.String("output", "../../docs/openapi.huma.generated.yaml", "output OpenAPI YAML path")
	flag.Parse()
	_, spec := contract.NewCartridgeAPI()
	data, err := spec.YAML()
	if err != nil {
		panic(fmt.Errorf("encode OpenAPI: %w", err))
	}
	if err := os.WriteFile(*output, data, 0o644); err != nil {
		panic(fmt.Errorf("write OpenAPI: %w", err))
	}
}
