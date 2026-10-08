package snmp

import (
	"context"
	"fmt"
	"time"

	gosnmp "github.com/gosnmp/gosnmp"
)

const (
	MarkerUnitOID = "1.3.6.1.2.1.43.10.2.1.3"
	MarkerLifeOID = "1.3.6.1.2.1.43.10.2.1.4"
)

type GoSNMPClient struct {
	client *gosnmp.GoSNMP
}

func NewGoSNMPClient(cfg Config) (*GoSNMPClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	client := &gosnmp.GoSNMP{
		Target:         cfg.Host,
		Port:           cfg.Port,
		Timeout:        time.Duration(cfg.TimeoutSeconds) * time.Second,
		Retries:        cfg.Retries,
		MaxRepetitions: uint32(cfg.MaxRepetitions),
		Version:        gosnmp.Version2c,
		Community:      cfg.Community,
	}
	if cfg.Version == Version3 {
		client.Version = gosnmp.Version3
		client.MsgFlags = gosnmp.NoAuthNoPriv
		client.SecurityParameters = &gosnmp.UsmSecurityParameters{UserName: cfg.Username}
		security := client.SecurityParameters.(*gosnmp.UsmSecurityParameters)
		if cfg.AuthProtocol != "" {
			protocol, err := authProtocol(cfg.AuthProtocol)
			if err != nil {
				return nil, err
			}
			security.AuthenticationProtocol = protocol
			security.AuthenticationPassphrase = cfg.AuthPassphrase
			client.MsgFlags = gosnmp.AuthNoPriv
		}
		if cfg.PrivProtocol != "" {
			protocol, err := privProtocol(cfg.PrivProtocol)
			if err != nil {
				return nil, err
			}
			security.PrivacyProtocol = protocol
			security.PrivacyPassphrase = cfg.PrivPassphrase
			client.MsgFlags = gosnmp.AuthPriv
		}
	}
	return &GoSNMPClient{client: client}, nil
}

func (c *GoSNMPClient) Connect() error { return c.client.Connect() }

func (c *GoSNMPClient) Get(ctx context.Context, oids []string) ([]VarBind, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := c.client.Get(oids)
	if err != nil {
		return nil, fmt.Errorf("SNMP GET: %w", err)
	}
	return convertPDUs(result.Variables), nil
}

func (c *GoSNMPClient) Walk(ctx context.Context, oid string) ([]VarBind, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var variables []gosnmp.SnmpPDU
	if err := c.client.Walk(oid, func(pdu gosnmp.SnmpPDU) error {
		variables = append(variables, pdu)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("SNMP WALK %s: %w", oid, err)
	}
	return convertPDUs(variables), nil
}

func (c *GoSNMPClient) Close() error {
	if c.client.Conn == nil {
		return nil
	}
	return c.client.Conn.Close()
}

func convertPDUs(pdus []gosnmp.SnmpPDU) []VarBind {
	result := make([]VarBind, 0, len(pdus))
	for _, pdu := range pdus {
		result = append(result, VarBind{OID: pdu.Name, Type: pdu.Type.String(), Value: jsonValue(pdu.Value)})
	}
	return result
}

func jsonValue(value any) any {
	switch value := value.(type) {
	case []byte:
		return fmt.Sprintf("0x%x", value)
	default:
		return value
	}
}

func authProtocol(name string) (gosnmp.SnmpV3AuthProtocol, error) {
	switch name {
	case "MD5":
		return gosnmp.MD5, nil
	case "SHA":
		return gosnmp.SHA, nil
	default:
		return gosnmp.NoAuth, fmt.Errorf("unsupported SNMPv3 auth protocol %q", name)
	}
}

func privProtocol(name string) (gosnmp.SnmpV3PrivProtocol, error) {
	switch name {
	case "DES":
		return gosnmp.DES, nil
	case "AES", "AES128":
		return gosnmp.AES, nil
	default:
		return gosnmp.NoPriv, fmt.Errorf("unsupported SNMPv3 privacy protocol %q", name)
	}
}
