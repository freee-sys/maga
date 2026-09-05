// Package snmp fetches basic device identity over SNMP (v1/v2c). It
// exposes a small Client interface so the discovery orchestrator can be
// tested against a fake, and a gosnmp-backed implementation for real use.
package snmp

import (
	"context"
	"errors"
)

// ErrTimeout indicates the device never responded (the common case for a
// wrong community string, since agents typically drop silently rather
// than sending an explicit rejection). Callers use errors.Is to tell this
// apart from a malformed/error response, which is a different discovery
// result classification.
var ErrTimeout = errors.New("snmp: request timed out")

// Identity is the handful of standard MIB-II fields discovery needs per
// device: sysDescr, sysObjectID, sysUpTime, and — most importantly —
// sysName, which is what rules match hostnames against.
type Identity struct {
	SysDescr    string
	SysObjectID string
	SysUpTime   uint32
	SysName     string
}

// Version is the SNMP protocol version to speak. v3 is out of scope for
// Phase 1 (see domain.DiscoveryJob's snmp_version constraint).
type Version string

const (
	VersionV1  Version = "v1"
	VersionV2c Version = "v2c"
)

// Client fetches one device's identity over SNMP. target may be a bare
// host ("10.0.0.5") or "host:port"; a bare host uses the standard SNMP
// port (161).
type Client interface {
	GetIdentity(ctx context.Context, target, community string, version Version) (Identity, error)
}
