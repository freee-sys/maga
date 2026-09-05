package snmp_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"

	"netcluster/internal/snmp"
	"netcluster/internal/snmp/snmptest"
)

func testValues() map[string]snmptest.OIDValue {
	return map[string]snmptest.OIDValue{
		"1.3.6.1.2.1.1.1.0": {Type: gosnmp.OctetString, Value: []byte("Cisco IOS switch")},
		"1.3.6.1.2.1.1.2.0": {Type: gosnmp.ObjectIdentifier, Value: "1.3.6.1.4.1.9.1.1"},
		"1.3.6.1.2.1.1.3.0": {Type: gosnmp.TimeTicks, Value: uint32(123456)},
		"1.3.6.1.2.1.1.5.0": {Type: gosnmp.OctetString, Value: []byte("sw-dc1-core-01")},
	}
}

func TestGoSNMPClient_GetIdentity_ReturnsValuesFromAgent(t *testing.T) {
	agent := snmptest.NewAgent(t, "public", testValues())
	client := snmp.NewGoSNMPClient(2*time.Second, 1)

	identity, err := client.GetIdentity(context.Background(), agent.Addr(), "public", snmp.VersionV2c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.SysName != "sw-dc1-core-01" {
		t.Errorf("expected sysName sw-dc1-core-01, got %q", identity.SysName)
	}
	if identity.SysDescr != "Cisco IOS switch" {
		t.Errorf("expected sysDescr Cisco IOS switch, got %q", identity.SysDescr)
	}
	if identity.SysObjectID != "1.3.6.1.4.1.9.1.1" {
		t.Errorf("expected sysObjectID 1.3.6.1.4.1.9.1.1, got %q", identity.SysObjectID)
	}
	if identity.SysUpTime != 123456 {
		t.Errorf("expected sysUpTime 123456, got %d", identity.SysUpTime)
	}
}

func TestGoSNMPClient_GetIdentity_WrongCommunityTimesOut(t *testing.T) {
	agent := snmptest.NewAgent(t, "public", testValues())
	client := snmp.NewGoSNMPClient(300*time.Millisecond, 0)

	_, err := client.GetIdentity(context.Background(), agent.Addr(), "wrong-community", snmp.VersionV2c)
	if err == nil {
		t.Fatal("expected timeout error for wrong community, got nil")
	}
	if !errors.Is(err, snmp.ErrTimeout) {
		t.Errorf("expected errors.Is(err, snmp.ErrTimeout), got %v", err)
	}
}

func TestGoSNMPClient_GetIdentity_UnreachableHostTimesOut(t *testing.T) {
	client := snmp.NewGoSNMPClient(300*time.Millisecond, 0)

	// TEST-NET-1 (RFC 5737): reserved for documentation, guaranteed unreachable.
	_, err := client.GetIdentity(context.Background(), "192.0.2.1", "public", snmp.VersionV2c)
	if err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
	if !errors.Is(err, snmp.ErrTimeout) {
		t.Errorf("expected errors.Is(err, snmp.ErrTimeout), got %v", err)
	}
}
