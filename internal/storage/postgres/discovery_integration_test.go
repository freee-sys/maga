//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gosnmp/gosnmp"

	"netcluster/internal/assignment"
	"netcluster/internal/clusters"
	"netcluster/internal/discovery"
	"netcluster/internal/domain"
	"netcluster/internal/snmp"
	"netcluster/internal/snmp/snmptest"
	"netcluster/internal/storage/postgres"
)

func snmpIdentityValues(sysName string) map[string]snmptest.OIDValue {
	return map[string]snmptest.OIDValue{
		"1.3.6.1.2.1.1.1.0": {Type: gosnmp.OctetString, Value: []byte("fake device")},
		"1.3.6.1.2.1.1.2.0": {Type: gosnmp.ObjectIdentifier, Value: "1.3.6.1.4.1.9.1.1"},
		"1.3.6.1.2.1.1.3.0": {Type: gosnmp.TimeTicks, Value: uint32(1000)},
		"1.3.6.1.2.1.1.5.0": {Type: gosnmp.OctetString, Value: []byte(sysName)},
	}
}

// TestDiscovery_EndToEnd_ScanTwoFakeHostsAssignsAndRecordsHistory runs the
// full Phase-1 pipeline for real: a discovery job scans two fake SNMP
// agents (real UDP wire protocol, no Docker needed for SNMP) plus one
// unreachable target, against a real Postgres database (via
// testcontainers) with a real rule seeded ahead of time. It verifies
// per-IP results, the resulting cluster, the element's assignment, and
// the assignment_history row — end to end, matching how discovery, the
// rule engine, and assignment are actually wired together in production.
func TestDiscovery_EndToEnd_ScanTwoFakeHostsAssignsAndRecordsHistory(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)

	ruleRepo := postgres.NewRuleRepo(pool)
	elementRepo := postgres.NewElementRepo(pool)
	clusterRepo := postgres.NewClusterRepo(pool)
	historyRepo := postgres.NewHistoryRepo(pool)
	jobRepo := postgres.NewDiscoveryJobRepo(pool)
	resultRepo := postgres.NewDiscoveryResultRepo(pool)

	_, err := ruleRepo.Create(ctx, domain.Rule{
		Name: "core switches", Priority: 10, IsEnabled: true,
		Pattern:            `^sw-(?P<site>[a-z0-9]+)-core-\d+$`,
		ClusterKeyTemplate: "{site}-core",
	})
	if err != nil {
		t.Fatalf("failed to seed rule: %v", err)
	}

	// Distinct loopback IPs so each agent is a distinguishable "device" —
	// the whole 127.0.0.0/8 range is loopback, and network_elements has a
	// real UNIQUE constraint on ip_address that this needs to respect.
	agent1 := snmptest.NewAgentOnIP(t, "127.0.0.11", "public", snmpIdentityValues("sw-dc1-core-01"))
	agent2 := snmptest.NewAgentOnIP(t, "127.0.0.12", "public", snmpIdentityValues("sw-dc2-core-01"))

	assignSvc := assignment.NewService(elementRepo, clusterRepo, historyRepo)
	_ = clusters.NewService(clusterRepo) // exercised indirectly via assignSvc; constructed here to prove the Repo interface is satisfied

	orch := discovery.NewOrchestrator(
		snmp.NewGoSNMPClient(500*time.Millisecond, 0),
		jobRepo, resultRepo, elementRepo, ruleRepo, assignSvc,
		5,
	)

	// target_ips is a real `inet[]` column, which can't hold a "host:port"
	// pair — production targets are always bare IPs on the standard SNMP
	// port. The fake agents bind ephemeral ports, so the job row is
	// created with placeholder IPs (proving persistence works end to end)
	// and the in-memory copy actually handed to the orchestrator is
	// swapped to the real agent addresses.
	job, err := jobRepo.Create(ctx, domain.DiscoveryJob{
		InputSpec:     "fake-agents",
		TargetIPs:     []string{"127.0.0.11", "127.0.0.12", "192.0.2.1"},
		SNMPVersion:   domain.SNMPVersionV2c,
		SNMPCommunity: "public",
	})
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	scanJob := job
	scanJob.TargetIPs = []string{agent1.Addr(), agent2.Addr(), "192.0.2.1:161"}
	if err := orch.Run(ctx, scanJob); err != nil {
		t.Fatalf("orchestrator run failed: %v", err)
	}

	gotJob, err := jobRepo.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("failed to reload job: %v", err)
	}
	if gotJob.Status != domain.DiscoveryJobStatusCompleted {
		t.Fatalf("expected job status completed, got %q", gotJob.Status)
	}

	results, err := resultRepo.ListByJob(ctx, job.ID, 0, 0)
	if err != nil {
		t.Fatalf("failed to list results: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 per-IP results, got %d", len(results))
	}
	var successCount, noResponseCount int
	for _, r := range results {
		switch r.Status {
		case domain.DiscoveryResultStatusSuccess:
			successCount++
		case domain.DiscoveryResultStatusNoResponse:
			noResponseCount++
		}
	}
	if successCount != 2 {
		t.Errorf("expected 2 successful results, got %d", successCount)
	}
	if noResponseCount != 1 {
		t.Errorf("expected 1 no_response result, got %d", noResponseCount)
	}

	clusterList, err := clusterRepo.List(ctx)
	if err != nil {
		t.Fatalf("failed to list clusters: %v", err)
	}
	byKey := map[string]domain.ClusterWithCount{}
	for _, c := range clusterList {
		byKey[c.LookupKey] = c
	}
	if _, ok := byKey["dc1-core"]; !ok {
		t.Error("expected cluster dc1-core to exist")
	}
	if _, ok := byKey["dc2-core"]; !ok {
		t.Error("expected cluster dc2-core to exist")
	}

	// Confirm the element for agent1's host actually landed in dc1-core
	// and has a history row explaining why.
	var elementID uuid.UUID
	for _, r := range results {
		if r.SysName == "sw-dc1-core-01" {
			elementID = *r.ElementID
		}
	}
	if elementID == uuid.Nil {
		t.Fatal("expected to find the element for sw-dc1-core-01")
	}
	el, err := elementRepo.Get(ctx, elementID)
	if err != nil {
		t.Fatalf("failed to reload element: %v", err)
	}
	if el.AssignmentStatus != domain.AssignmentStatusAssigned || el.ClusterID == nil {
		t.Fatalf("expected element to be assigned, got %+v", el)
	}

	history, err := historyRepo.ListByElement(ctx, elementID)
	if err != nil {
		t.Fatalf("failed to list history: %v", err)
	}
	if len(history) != 1 || history[0].TriggerSource != domain.TriggerSourceDiscovery {
		t.Fatalf("expected one discovery-triggered history row, got %+v", history)
	}
}
