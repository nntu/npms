package ingestion

import (
	"context"
	"fmt"
	"time"

	"npms/backend/internal/counter"
	"npms/backend/internal/repository"
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
	reading, readErr := (SNMPReader{Client: client, OID: definition.OID, Instance: definition.Instance, Quality: quality}).Read(ctx)
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
