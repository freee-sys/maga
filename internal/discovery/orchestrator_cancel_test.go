package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"netcluster/internal/domain"
	"netcluster/internal/snmp"
)

func TestOrchestrator_Run_CancelledContextEndsJobCancelled(t *testing.T) {
	snmpClient := newFakeSNMPClient()
	snmpClient.delay = 200 * time.Millisecond
	snmpClient.byIP["10.0.0.1"] = snmp.Identity{SysName: "sw-x-01"}
	snmpClient.byIP["10.0.0.2"] = snmp.Identity{SysName: "sw-x-02"}
	snmpClient.byIP["10.0.0.3"] = snmp.Identity{SysName: "sw-x-03"}

	jobs := &fakeJobRepo{}
	results := &fakeResultRepo{}
	elements := newFakeElementUpserter()
	assigner := &fakeAssigner{}
	orch := newTestOrchestrator(snmpClient, jobs, results, elements, assigner, 1)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	job := domain.DiscoveryJob{
		ID:            uuid.New(),
		TargetIPs:     []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		SNMPCommunity: "public", SNMPVersion: domain.SNMPVersionV2c,
	}

	// Run must still return successfully (able to record the final status)
	// even though the context it was given is cancelled.
	if err := orch.Run(ctx, job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	last := jobs.statuses[len(jobs.statuses)-1]
	if last != domain.DiscoveryJobStatusCancelled {
		t.Fatalf("expected final status cancelled, got %v", jobs.statuses)
	}
}
