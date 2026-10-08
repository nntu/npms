package discovery

import (
	"context"
	"fmt"
	"strings"

	"npms/backend/internal/snmp"
)

const (
	SysDescrOID  = "1.3.6.1.2.1.1.1.0"
	SysObjectOID = "1.3.6.1.2.1.1.2.0"
	SysNameOID   = "1.3.6.1.2.1.1.5.0"
	SerialOID    = "1.3.6.1.2.1.43.5.1.1.17"
)

type Result struct {
	Address     string `json:"address"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SysObjectID string `json:"sys_object_id"`
	Serial      string `json:"serial"`
}

type ClientFactory func(snmp.Config) (snmp.Client, error)

func Probe(ctx context.Context, config snmp.Config, factory ClientFactory) (Result, error) {
	if factory == nil {
		return Result{}, fmt.Errorf("SNMP client factory is required")
	}
	client, err := factory(config)
	if err != nil {
		return Result{}, err
	}
	defer client.Close()
	values, err := client.Get(ctx, []string{SysDescrOID, SysObjectOID, SysNameOID, SerialOID})
	if err != nil {
		return Result{}, err
	}
	result := Result{Address: config.Host}
	result.Description = lookup(values, SysDescrOID)
	result.SysObjectID = lookup(values, SysObjectOID)
	result.Name = lookup(values, SysNameOID)
	result.Serial = lookup(values, SerialOID)
	return result, nil
}

func lookup(values []snmp.VarBind, oid string) string {
	for _, value := range values {
		if strings.TrimPrefix(value.OID, ".") != oid {
			continue
		}
		if bytes, ok := value.Value.([]byte); ok {
			return string(bytes)
		}
		return fmt.Sprint(value.Value)
	}
	return ""
}
