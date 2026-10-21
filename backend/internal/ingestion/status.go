package ingestion

import (
	"context"
	"fmt"
	"strings"
	"time"

	"npms/backend/internal/repository"
	"npms/backend/internal/snmp"
)

const sysNameOID = "1.3.6.1.2.1.1.5.0"

type EndpointStore interface {
	ListDeviceEndpoints(context.Context, string) ([]repository.DeviceEndpoint, error)
	MarkEndpointSuccess(context.Context, string, time.Time) error
	MarkDeviceOffline(context.Context, string) error
}

type ClientFactory func(snmp.Config) (snmp.Client, error)

type StatusPoller struct {
	Store    EndpointStore
	Resolver ConfigResolver
	Factory  ClientFactory
}

func NewStatusPoller(store EndpointStore, resolver ConfigResolver, factory ClientFactory) (*StatusPoller, error) {
	if store == nil {
		return nil, fmt.Errorf("endpoint store is required")
	}
	if resolver.Store == nil || resolver.Box == nil {
		return nil, fmt.Errorf("credential resolver is required")
	}
	if factory == nil {
		return nil, fmt.Errorf("SNMP client factory is required")
	}
	return &StatusPoller{Store: store, Resolver: resolver, Factory: factory}, nil
}

func (p *StatusPoller) PollDevice(ctx context.Context, deviceID string) error {
	endpoints, err := p.Store.ListDeviceEndpoints(ctx, deviceID)
	if err != nil {
		return err
	}
	if len(endpoints) == 0 {
		return fmt.Errorf("device %s has no SNMP endpoint", deviceID)
	}
	var lastErr error
	for _, endpoint := range endpoints {
		if endpoint.Protocol != "" && endpoint.Protocol != "snmp" {
			continue
		}
		config, err := p.Resolver.Resolve(ctx, endpoint)
		if err != nil {
			lastErr = err
			continue
		}
		client, err := p.Factory(config)
		if err != nil {
			lastErr = err
			continue
		}
		values, readErr := client.Get(ctx, []string{sysNameOID})
		closeErr := client.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}
		var found bool
		for _, value := range values {
			if strings.TrimPrefix(value.OID, ".") == sysNameOID {
				if strings.EqualFold(value.Type, "NoSuchObject") || strings.EqualFold(value.Type, "NoSuchInstance") {
					readErr = fmt.Errorf("SNMP OID %s not supported by device", sysNameOID)
					break
				}
				found = true
				break
			}
		}
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if !found {
			lastErr = fmt.Errorf("SNMP response did not contain requested OID %s", sysNameOID)
			continue
		}
		if err := p.Store.MarkEndpointSuccess(ctx, endpoint.ID, time.Now().UTC()); err != nil {
			return err
		}
		return nil
	}
	if lastErr == nil {
		_ = p.Store.MarkDeviceOffline(ctx, deviceID)
		return fmt.Errorf("device %s has no supported SNMP endpoint", deviceID)
	}
	_ = p.Store.MarkDeviceOffline(ctx, deviceID)
	return fmt.Errorf("status poll device %s: %w", deviceID, lastErr)
}

func ConnectSNMP(config snmp.Config) (snmp.Client, error) {
	client, err := snmp.NewGoSNMPClient(config)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}
