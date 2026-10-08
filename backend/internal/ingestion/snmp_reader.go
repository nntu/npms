package ingestion

import (
	"context"
	"fmt"
	"strings"
	"time"

	"npms/backend/internal/counter"
	"npms/backend/internal/snmp"
)

// SNMPReader reads one already-resolved scalar/instance OID. Profile matching,
// marker joins and counter validation remain outside this transport adapter.
type SNMPReader struct {
	Client   snmp.Client
	OID      string
	Instance string
	Quality  counter.Quality
	EpochID  string
}

func (r SNMPReader) Read(ctx context.Context) (counter.Reading, error) {
	if r.Client == nil {
		return counter.Reading{}, fmt.Errorf("SNMP client is required")
	}
	base := strings.TrimPrefix(strings.TrimSpace(r.OID), ".")
	if base == "" {
		return counter.Reading{}, fmt.Errorf("SNMP OID is required")
	}
	requestOID := base
	if strings.TrimSpace(r.Instance) != "" {
		requestOID += "." + strings.Trim(strings.TrimSpace(r.Instance), ".")
	}
	values, err := r.Client.Get(ctx, []string{requestOID})
	if err != nil {
		return counter.Reading{}, err
	}
	for _, value := range values {
		if strings.TrimPrefix(value.OID, ".") != requestOID {
			continue
		}
		raw, ok := snmp.NumericValue(value.Value)
		if !ok {
			return counter.Reading{}, fmt.Errorf("SNMP value for %s is not an integer", requestOID)
		}
		quality := r.Quality
		if quality == "" {
			quality = counter.QualityUnverified
		}
		return counter.Reading{RawValue: raw, CollectedAt: time.Now().UTC(), Quality: quality, EpochID: r.EpochID}, nil
	}
	return counter.Reading{}, fmt.Errorf("SNMP response did not contain requested OID %s", requestOID)
}
