package ingestion

import (
	"context"
	"fmt"
	"time"

	"npms/backend/internal/counter"
	"npms/backend/internal/repository"
	"npms/backend/internal/snmp"
)

type CounterPoller struct {
	Resolver ConfigResolver
	Factory  ClientFactory
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
	life, err := client.Walk(ctx, definition.OID)
	if err != nil {
		return counter.Reading{}, fmt.Errorf("walk counter %q life column: %w", definition.Key, err)
	}
	units, err := client.Walk(ctx, definition.UnitOID)
	if err != nil {
		return counter.Reading{}, fmt.Errorf("walk counter %q unit column: %w", definition.Key, err)
	}
	rows := snmp.JoinMarkerRows(life, units, definition.OID, definition.UnitOID)
	if len(rows) == 0 {
		return counter.Reading{}, fmt.Errorf("walk counter %q returned no validated marker rows", definition.Key)
	}
	if len(rows) > 1 {
		return counter.Reading{}, fmt.Errorf("walk counter %q returned %d validated marker rows; explicit instance selection is required", definition.Key, len(rows))
	}
	return counter.Reading{RawValue: rows[0].Life, Quality: quality, EpochID: definition.Instance}, nil
}
