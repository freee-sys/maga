// Package snmptest is a minimal fake SNMP v2c agent: it listens on a
// real (loopback) UDP socket, decodes GET requests with gosnmp's own
// packet codec, and answers from a canned OID table. It exists so
// internal/snmp's real wire-protocol handling can be tested without
// Docker or an external simulator (like snmpsim) in CI.
package snmptest

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/gosnmp/gosnmp"
)

// OIDValue is one canned response value for a given OID.
type OIDValue struct {
	Type  gosnmp.Asn1BER
	Value any
}

// Agent is a fake SNMP v2c responder bound to an ephemeral loopback
// port. Only requests whose community matches are answered; anything
// else is silently dropped, matching how a real agent behaves on a bad
// community string (the client sees a timeout, not an explicit error).
type Agent struct {
	conn      *net.UDPConn
	community string

	mu     sync.Mutex
	values map[string]OIDValue
}

// NewAgent starts a fake agent bound to 127.0.0.1, seeded with values
// (keyed by OID, with or without a leading dot), and stops it
// automatically when t completes. Use NewAgentOnIP for a test that needs
// several distinguishable agents (e.g. exercising a real per-IP unique
// constraint) — the whole 127.0.0.0/8 range is loopback.
func NewAgent(t testing.TB, community string, values map[string]OIDValue) *Agent {
	t.Helper()
	return NewAgentOnIP(t, "127.0.0.1", community, values)
}

func NewAgentOnIP(t testing.TB, bindIP string, community string, values map[string]OIDValue) *Agent {
	t.Helper()

	a, err := NewStandaloneAgent(bindIP, 0, community, values)
	if err != nil {
		t.Fatalf("snmptest: %v", err)
	}
	t.Cleanup(a.Close)
	return a
}

// NewStandaloneAgent is NewAgentOnIP without a testing.TB and with an
// explicit port (0 for ephemeral), for use outside `go test` — e.g. a
// one-off local dev-seeding script that needs the real SNMP port (161)
// to match what production discovery actually targets. Callers are
// responsible for calling Close when done.
func NewStandaloneAgent(bindIP string, port int, community string, values map[string]OIDValue) (*Agent, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(bindIP), Port: port})
	if err != nil {
		return nil, fmt.Errorf("failed to open UDP socket on %s: %w", bindIP, err)
	}

	normalized := make(map[string]OIDValue, len(values))
	for oid, v := range values {
		normalized[normalizeOID(oid)] = v
	}

	a := &Agent{conn: conn, community: community, values: normalized}
	go a.serve()
	return a, nil
}

// Addr returns the "host:port" this agent is listening on, suitable to
// pass straight to snmp.Client.GetIdentity as the target.
func (a *Agent) Addr() string {
	return a.conn.LocalAddr().String()
}

// Close stops the agent and releases its UDP socket.
func (a *Agent) Close() {
	a.conn.Close()
}

func (a *Agent) serve() {
	buf := make([]byte, 4096)
	for {
		n, addr, err := a.conn.ReadFromUDP(buf)
		if err != nil {
			return // socket closed
		}
		packet := make([]byte, n)
		copy(packet, buf[:n])
		go a.respond(packet, addr)
	}
}

func (a *Agent) respond(request []byte, addr *net.UDPAddr) {
	decoder := &gosnmp.GoSNMP{Version: gosnmp.Version2c}
	pkt, err := decoder.SnmpDecodePacket(request)
	if err != nil || pkt.Community != a.community {
		return
	}

	a.mu.Lock()
	responsePDUs := make([]gosnmp.SnmpPDU, len(pkt.Variables))
	for i, reqPDU := range pkt.Variables {
		if v, ok := a.values[normalizeOID(reqPDU.Name)]; ok {
			responsePDUs[i] = gosnmp.SnmpPDU{Name: reqPDU.Name, Type: v.Type, Value: v.Value}
		} else {
			responsePDUs[i] = gosnmp.SnmpPDU{Name: reqPDU.Name, Type: gosnmp.NoSuchObject}
		}
	}
	a.mu.Unlock()

	encoder := &gosnmp.GoSNMP{Version: gosnmp.Version2c, Community: a.community}
	respPacket := encoder.MkSnmpPacket(gosnmp.GetResponse, responsePDUs, 0, 0)
	respPacket.RequestID = pkt.RequestID

	respBytes, err := respPacket.MarshalMsg()
	if err != nil {
		return
	}
	_, _ = a.conn.WriteToUDP(respBytes, addr)
}

func normalizeOID(oid string) string {
	if len(oid) > 0 && oid[0] == '.' {
		return oid[1:]
	}
	return oid
}
