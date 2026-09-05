package snmp

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

const (
	defaultPort = 161

	oidSysDescr    = "1.3.6.1.2.1.1.1.0"
	oidSysObjectID = "1.3.6.1.2.1.1.2.0"
	oidSysUpTime   = "1.3.6.1.2.1.1.3.0"
	oidSysName     = "1.3.6.1.2.1.1.5.0"
)

// GoSNMPClient fetches device identity over SNMP v2c using gosnmp.
type GoSNMPClient struct {
	Timeout time.Duration
	Retries int
}

func NewGoSNMPClient(timeout time.Duration, retries int) *GoSNMPClient {
	return &GoSNMPClient{Timeout: timeout, Retries: retries}
}

func (c *GoSNMPClient) GetIdentity(ctx context.Context, target, community string, version Version) (Identity, error) {
	host, port, err := splitHostPort(target)
	if err != nil {
		return Identity{}, fmt.Errorf("invalid target %q: %w", target, err)
	}

	gosnmpVersion, err := toGoSNMPVersion(version)
	if err != nil {
		return Identity{}, err
	}

	g := &gosnmp.GoSNMP{
		Target:    host,
		Port:      port,
		Community: community,
		Version:   gosnmpVersion,
		Timeout:   c.Timeout,
		Retries:   c.Retries,
		Context:   ctx,
	}
	if err := g.Connect(); err != nil {
		return Identity{}, fmt.Errorf("connect to %s: %w", target, err)
	}
	defer g.Conn.Close()

	result, err := g.Get([]string{oidSysDescr, oidSysObjectID, oidSysUpTime, oidSysName})
	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			return Identity{}, fmt.Errorf("snmp get %s: %w", target, ErrTimeout)
		}
		return Identity{}, fmt.Errorf("snmp get %s: %w", target, err)
	}
	if result.Error != gosnmp.NoError {
		return Identity{}, fmt.Errorf("snmp get %s: device returned error %s", target, result.Error.String())
	}

	var identity Identity
	for _, v := range result.Variables {
		switch normalizeOID(v.Name) {
		case oidSysDescr:
			identity.SysDescr = pduString(v)
		case oidSysObjectID:
			// gosnmp decodes ObjectIdentifier values with a leading dot
			// (e.g. ".1.3.6.1.4.1.9.1.1"); normalize like an OID name.
			identity.SysObjectID = normalizeOID(pduString(v))
		case oidSysName:
			identity.SysName = pduString(v)
		case oidSysUpTime:
			identity.SysUpTime = pduUint32(v)
		}
	}
	return identity, nil
}

func toGoSNMPVersion(v Version) (gosnmp.SnmpVersion, error) {
	switch v {
	case VersionV1:
		return gosnmp.Version1, nil
	case VersionV2c:
		return gosnmp.Version2c, nil
	default:
		return 0, fmt.Errorf("unsupported snmp version %q", v)
	}
}

func splitHostPort(target string) (string, uint16, error) {
	if host, portStr, err := net.SplitHostPort(target); err == nil {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port %q: %w", portStr, err)
		}
		return host, uint16(port), nil
	}
	return target, defaultPort, nil
}

func normalizeOID(oid string) string {
	if len(oid) > 0 && oid[0] == '.' {
		return oid[1:]
	}
	return oid
}

func pduString(v gosnmp.SnmpPDU) string {
	switch val := v.Value.(type) {
	case []byte:
		return string(val)
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

func pduUint32(v gosnmp.SnmpPDU) uint32 {
	switch val := v.Value.(type) {
	case uint32:
		return val
	case uint:
		return uint32(val)
	case int:
		return uint32(val)
	default:
		return 0
	}
}
