package ingestion

import (
	"context"
	"fmt"
	"strings"
	"time"

	"npms/backend/internal/counter"
	"npms/backend/internal/repository"
	"npms/backend/internal/snmp"
)

type CounterPoller struct {
	Resolver ConfigResolver
	Factory  ClientFactory
}

type DeviceCounterStore interface {
	ListDeviceEndpoints(context.Context, string) ([]repository.DeviceEndpoint, error)
	ListCounterDefinitions(context.Context, string) ([]repository.CounterDefinition, error)
}

// ReadDevice reads a configured counter without creating a polling run. It is
// used for point-in-time actions such as cartridge replacement.
func (p *CounterPoller) ReadDevice(ctx context.Context, deviceID string) (counter.Reading, repository.CounterDefinition, error) {
	store, ok := p.Resolver.Store.(DeviceCounterStore)
	if !ok {
		return counter.Reading{}, repository.CounterDefinition{}, fmt.Errorf("counter snapshot store is not configured")
	}
	endpoints, err := store.ListDeviceEndpoints(ctx, deviceID)
	if err != nil {
		return counter.Reading{}, repository.CounterDefinition{}, err
	}
	var endpoint repository.DeviceEndpoint
	for _, candidate := range endpoints {
		if candidate.IsPrimary {
			endpoint = candidate
			break
		}
	}
	if endpoint.ID == "" && len(endpoints) > 0 {
		endpoint = endpoints[0]
	}
	if endpoint.ID == "" {
		return counter.Reading{}, repository.CounterDefinition{}, fmt.Errorf("device %s has no SNMP endpoint", deviceID)
	}
	definitions, err := store.ListCounterDefinitions(ctx, deviceID)
	if err != nil {
		return counter.Reading{}, repository.CounterDefinition{}, err
	}
	for _, definition := range definitions {
		if definition.Key == "marker_life" || definition.SemanticType == "marker_life" {
			reading, readErr := p.Read(ctx, endpoint, definition)
			return reading, definition, readErr
		}
	}
	return counter.Reading{}, repository.CounterDefinition{}, fmt.Errorf("device %s has no marker counter definition", deviceID)
}

func NewCounterPoller(resolver ConfigResolver, factory ClientFactory) (*CounterPoller, error) {
	if resolver.Store == nil || resolver.Box == nil {
		return nil, fmt.Errorf("credential resolver is required")
	}
	if factory == nil {
		return nil, fmt.Errorf("SNMP client factory is required")
	}
	return &CounterPoller{Resolver: resolver, Factory: factory}, nil
}

func (p *CounterPoller) Read(ctx context.Context, endpoint repository.DeviceEndpoint, definition repository.CounterDefinition) (counter.Reading, error) {
	config, err := p.Resolver.Resolve(ctx, endpoint)
	if err != nil {
		return counter.Reading{}, err
	}
	client, err := p.Factory(config)
	if err != nil {
		return counter.Reading{}, err
	}
	quality := counter.QualityUnverified
	if definition.Verified {
		quality = counter.QualityValid
	}
	var reading counter.Reading
	var readErr error
	if definition.Mode == "walk" {
		reading, readErr = readWalkCounter(ctx, client, definition, quality)
	} else {
		reading, readErr = (SNMPReader{Client: client, OID: definition.OID, Instance: definition.Instance, Quality: quality}).Read(ctx)
	}
	closeErr := client.Close()
	if readErr != nil {
		return counter.Reading{}, readErr
	}
	if closeErr != nil {
		return counter.Reading{}, closeErr
	}
	reading.CollectedAt = time.Now().UTC()
	return reading, nil
}

func readWalkCounter(ctx context.Context, client snmp.Client, definition repository.CounterDefinition, quality counter.Quality) (counter.Reading, error) {
	if definition.UnitOID == "" {
		return counter.Reading{}, fmt.Errorf("walk counter %q requires a unit OID", definition.Key)
	}
	if definition.Selection != "" && definition.Selection != "validated_marker_rows" {
		return counter.Reading{}, fmt.Errorf("unsupported walk counter selection %q", definition.Selection)
	}
	if strings.TrimSpace(definition.Instance) != "" {
		wanted := strings.Trim(strings.TrimSpace(definition.Instance), ".")
		if row, ok, getErr := snmp.TryReadMarkerByInstance(ctx, client, definition.OID, definition.UnitOID, wanted); getErr == nil && ok {
			return counter.Reading{RawValue: row.Life, Quality: quality, EpochID: wanted}, nil
		}
	}
	life, err := client.Walk(ctx, definition.OID)
	if err != nil {
		if val, epoch, fbErr := tryFallbackTotalCounters(ctx, client); fbErr == nil {
			return counter.Reading{RawValue: val, Quality: quality, EpochID: epoch}, nil
		}
		return counter.Reading{}, fmt.Errorf("walk counter %q life column: %w", definition.Key, err)
	}
	units, _ := client.Walk(ctx, definition.UnitOID)
	rows := snmp.JoinMarkerRows(life, units, definition.OID, definition.UnitOID)
	if len(rows) == 0 {
		// Fallback 1: Extract life value from life column directly if device omitted unit column
		if len(life) > 0 {
			wanted := strings.TrimPrefix(strings.TrimSpace(definition.Instance), ".")
			for _, vb := range life {
				if snmp.IsExceptionVarBind(vb) {
					continue
				}
				inst, ok := snmp.TableInstance(vb.OID, definition.OID)
				if !ok {
					continue
				}
				val, ok := snmp.NumericValue(vb.Value)
				if !ok {
					continue
				}
				if wanted == "" || inst == wanted {
					return counter.Reading{RawValue: val, Quality: quality, EpochID: inst}, nil
				}
			}
		}

		// Fallback 2: Try vendor total page counters (HP, Brother, Ricoh, Canon, Xerox, MIB-II)
		if val, epoch, fbErr := tryFallbackTotalCounters(ctx, client); fbErr == nil {
			return counter.Reading{RawValue: val, Quality: quality, EpochID: epoch}, nil
		}

		return counter.Reading{}, fmt.Errorf("walk counter %q returned no validated marker rows", definition.Key)
	}
	if definition.Instance != "" {
		wanted := strings.TrimPrefix(definition.Instance, ".")
		for _, row := range rows {
			if row.Instance == wanted {
				return counter.Reading{RawValue: row.Life, Quality: quality, EpochID: wanted}, nil
			}
		}
		return counter.Reading{}, fmt.Errorf("walk counter %q did not return configured instance %q", definition.Key, definition.Instance)
	}
	if len(rows) > 1 {
		if definition.Unit != "" {
			var matching []snmp.MarkerRow
			for _, row := range rows {
				if snmp.CounterUnitName(row.Unit) == definition.Unit {
					matching = append(matching, row)
				}
			}
			if len(matching) == 1 {
				return counter.Reading{RawValue: matching[0].Life, Quality: quality, EpochID: matching[0].Instance}, nil
			}
		}
		return counter.Reading{}, fmt.Errorf("walk counter %q returned %d validated marker rows; explicit instance selection is required", definition.Key, len(rows))
	}
	epochID := rows[0].Instance
	if definition.Instance != "" {
		epochID = definition.Instance
	}
	return counter.Reading{RawValue: rows[0].Life, Quality: quality, EpochID: epochID}, nil
}

func tryFallbackTotalCounters(ctx context.Context, client snmp.Client) (int64, string, error) {
	candidates := []struct {
		oid   string
		epoch string
	}{
		// Standard Printer-MIB Marker Life instance 1.1 direct GET
		{oid: "1.3.6.1.2.1.43.10.2.1.4.1.1", epoch: "1.1"},
		// HP LaserJet Total Engine Page Count
		{oid: "1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.5.0", epoch: "hp_engine_total"},
		// Brother Total Page Count
		{oid: "1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.8.0", epoch: "brother_total"},
		{oid: "1.3.6.1.4.1.2435.2.3.9.1.1.7.0", epoch: "brother_total_alt"},
		// Ricoh System Total Counter
		{oid: "1.3.6.1.4.1.367.3.2.1.2.19.1.0", epoch: "ricoh_total"},
		// Xerox Total Impressions
		{oid: "1.3.6.1.4.1.253.8.53.13.2.1.6.1.20.1", epoch: "xerox_total"},
		// Canon Total Counter
		{oid: "1.3.6.1.4.1.1602.1.11.1.3.1.4.1", epoch: "canon_total"},
	}

	for _, c := range candidates {
		vbs, err := client.Get(ctx, []string{c.oid})
		if err != nil || len(vbs) == 0 || snmp.IsExceptionVarBind(vbs[0]) {
			continue
		}
		val, ok := snmp.NumericValue(vbs[0].Value)
		if ok && val >= 0 {
			return val, c.epoch, nil
		}
	}
	return 0, "", fmt.Errorf("no fallback counter responded")
}
