package security

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type SNMPSecret struct {
	Version        string `json:"version"`
	Community      string `json:"community,omitempty"`
	Username       string `json:"username,omitempty"`
	AuthProtocol   string `json:"auth_protocol,omitempty"`
	AuthPassphrase string `json:"auth_passphrase,omitempty"`
	PrivProtocol   string `json:"priv_protocol,omitempty"`
	PrivPassphrase string `json:"priv_passphrase,omitempty"`
}

func (s SNMPSecret) Validate() error {
	switch s.Version {
	case "2c":
		if s.Community == "" {
			return fmt.Errorf("SNMP v2c community is required")
		}
	case "3":
		if s.Username == "" {
			return fmt.Errorf("SNMPv3 username is required")
		}
		if s.AuthProtocol != "" && s.AuthPassphrase == "" {
			return fmt.Errorf("SNMPv3 auth passphrase is required")
		}
		if s.PrivProtocol != "" && s.PrivPassphrase == "" {
			return fmt.Errorf("SNMPv3 privacy passphrase is required")
		}
	default:
		return fmt.Errorf("unsupported SNMP version %q", s.Version)
	}
	return nil
}

func EncodeSNMPSecret(secret SNMPSecret) ([]byte, error) {
	if err := secret.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(secret)
}

func DecodeSNMPSecret(data []byte) (SNMPSecret, error) {
	var secret SNMPSecret
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&secret); err != nil {
		return SNMPSecret{}, fmt.Errorf("decode SNMP secret: %w", err)
	}
	if err := secret.Validate(); err != nil {
		return SNMPSecret{}, err
	}
	return secret, nil
}
