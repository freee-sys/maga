package discovery_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"netcluster/internal/discovery"
	"netcluster/internal/domain"
	"netcluster/internal/ruleengine"
	"netcluster/internal/snmp"
)

// fakeSNMPClient answers GetIdentity purely from an in-memory table, no
// network — this is what lets orchestrator pooling/classification logic
// be tested fast and deterministically. Real wire-protocol behavior is
// covered separately in internal/snmp against snmptest's fake UDP agent.
type fakeSNMPClient struct {
	mu          sync.Mutex
	byIP        map[string]snmp.Identity
	errByIP     map[string]error
	concurrent  int32
	maxObserved int32
	delay       time.Duration
}

func newFakeSNMPClient() *fakeSNMPClient {
	return &fakeSNMPClient{byIP: map[string]snmp.Identity{}, errByIP: map[string]error{}}
}

func (f *fakeSNMPClient) GetIdentity(ctx context.Context, target, community string, version snmp.Version) (snmp.Identity, error) {
	cur := atomic.AddInt32(&f.concurrent, 1)
	defer atomic.AddInt32(&f.concurrent, -1)
	for {
		max := atomic.LoadInt32(&f.maxObserved)
		if cur <= max || atomic.CompareAndSwapInt32(&f.maxObserved, max, cur) {
			break
		}
	}

	if f.delay > 0 {
		time.Sleep(f.delay)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.errByIP[target]; ok {
		return snmp.Identity{}, err
	}
	if id, ok := f.byIP[target]; ok {
		return id, nil
	}
	return snmp.Identity{}, errors.New("unconfigured target")
}

type fakeJobRepo struct {
	mu       sync.Mutex
	statuses []string
	errMsgs  []string
}

func (f *fakeJobRepo) UpdateStatus(ctx context.Context, jobID uuid.UUID, status string, errorMessage string) error {
	// A real DB call would fail against a cancelled context; asserting
	// that here catches an orchestrator that reuses the job's (possibly
	// cancelled) context for its final status write instead of a fresh one.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statuses = append(f.statuses, status)
	f.errMsgs = append(f.errMsgs, errorMessage)
	return nil
}

type fakeResultRepo struct {
	mu      sync.Mutex
	results []domain.DiscoveryJobResult
}

func (f *fakeResultRepo) InsertResult(ctx context.Context, r domain.DiscoveryJobResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results = append(f.results, r)
	return nil
}

func (f *fakeResultRepo) byStatus(status string) []domain.DiscoveryJobResult {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.DiscoveryJobResult
	for _, r := range f.results {
		if r.Status == status {
			out = append(out, r)
		}
	}
	return out
}

type fakeElementUpserter struct {
	mu    sync.Mutex
	byIP  map[string]domain.NetworkElement
	calls int
}

func newFakeElementUpserter() *fakeElementUpserter {
	return &fakeElementUpserter{byIP: map[string]domain.NetworkElement{}}
}

func (f *fakeElementUpserter) UpsertDiscovered(ctx context.Context, ip string, identity snmp.Identity) (domain.NetworkElement, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	el := domain.NetworkElement{ID: uuid.New(), IPAddress: ip, SysName: identity.SysName}
	f.byIP[ip] = el
	return el, nil
}

type fakeRuleLister struct{}

func (fakeRuleLister) ListEnabledOrdered(ctx context.Context) ([]domain.Rule, error) {
	return []domain.Rule{{IsEnabled: true, Pattern: `^sw-(?P<site>[a-z0-9]+)-\d+$`, ClusterKeyTemplate: "{site}"}}, nil
}

type fakeAssigner struct {
	mu    sync.Mutex
	calls []string // hostnames assigned
}

func (f *fakeAssigner) Assign(ctx context.Context, elementID uuid.UUID, hostname string, rules []domain.Rule, trigger string) (*ruleengine.MatchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, hostname)
	return &ruleengine.MatchResult{}, nil
}

func newTestOrchestrator(snmpClient *fakeSNMPClient, jobs *fakeJobRepo, results *fakeResultRepo, elements *fakeElementUpserter, assigner *fakeAssigner, concurrency int) *discovery.Orchestrator {
	return discovery.NewOrchestrator(snmpClient, jobs, results, elements, fakeRuleLister{}, assigner, concurrency)
}

func TestOrchestrator_Run_ClassifiesEachTargetAndMarksJobCompleted(t *testing.T) {
	snmpClient := newFakeSNMPClient()
	snmpClient.byIP["10.0.0.1"] = snmp.Identity{SysName: "sw-dc1-01"}
	snmpClient.errByIP["10.0.0.2"] = snmp.ErrTimeout
	snmpClient.errByIP["10.0.0.3"] = errors.New("device returned malformed response")

	jobs := &fakeJobRepo{}
	results := &fakeResultRepo{}
	elements := newFakeElementUpserter()
	assigner := &fakeAssigner{}
	orch := newTestOrchestrator(snmpClient, jobs, results, elements, assigner, 5)

	job := domain.DiscoveryJob{
		ID: uuid.New(), TargetIPs: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		SNMPVersion: domain.SNMPVersionV2c, SNMPCommunity: "public",
	}

	if err := orch.Run(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs.statuses) < 2 || jobs.statuses[0] != domain.DiscoveryJobStatusRunning {
		t.Fatalf("expected job to be marked running first, got %v", jobs.statuses)
	}
	if last := jobs.statuses[len(jobs.statuses)-1]; last != domain.DiscoveryJobStatusCompleted {
		t.Fatalf("expected job to end completed, got %v", jobs.statuses)
	}

	if got := results.byStatus(domain.DiscoveryResultStatusSuccess); len(got) != 1 {
		t.Errorf("expected 1 success result, got %d", len(got))
	}
	if got := results.byStatus(domain.DiscoveryResultStatusNoResponse); len(got) != 1 {
		t.Errorf("expected 1 no_response result, got %d", len(got))
	}
	if got := results.byStatus(domain.DiscoveryResultStatusSNMPError); len(got) != 1 {
		t.Errorf("expected 1 snmp_error result, got %d", len(got))
	}

	if elements.calls != 1 {
		t.Errorf("expected exactly 1 element upsert (only the successful host), got %d", elements.calls)
	}
	if len(assigner.calls) != 1 || assigner.calls[0] != "sw-dc1-01" {
		t.Errorf("expected assignment triggered once for sw-dc1-01, got %v", assigner.calls)
	}
}

func TestOrchestrator_Run_HostFailureDoesNotAbortOtherHosts(t *testing.T) {
	snmpClient := newFakeSNMPClient()
	snmpClient.errByIP["10.0.0.1"] = snmp.ErrTimeout
	snmpClient.byIP["10.0.0.2"] = snmp.Identity{SysName: "sw-dc1-02"}

	jobs := &fakeJobRepo{}
	results := &fakeResultRepo{}
	elements := newFakeElementUpserter()
	assigner := &fakeAssigner{}
	orch := newTestOrchestrator(snmpClient, jobs, results, elements, assigner, 5)

	job := domain.DiscoveryJob{ID: uuid.New(), TargetIPs: []string{"10.0.0.1", "10.0.0.2"}, SNMPCommunity: "public", SNMPVersion: domain.SNMPVersionV2c}

	if err := orch.Run(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jobs.statuses[len(jobs.statuses)-1] != domain.DiscoveryJobStatusCompleted {
		t.Fatalf("expected job to complete despite one host failing, got %v", jobs.statuses)
	}
	if len(results.results) != 2 {
		t.Fatalf("expected results recorded for both hosts, got %d", len(results.results))
	}
}

func TestOrchestrator_Run_RespectsConcurrencyLimit(t *testing.T) {
	snmpClient := newFakeSNMPClient()
	snmpClient.delay = 50 * time.Millisecond
	var targets []string
	for i := 1; i <= 10; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i)
		targets = append(targets, ip)
		snmpClient.byIP[ip] = snmp.Identity{SysName: "sw-x-01"}
	}

	jobs := &fakeJobRepo{}
	results := &fakeResultRepo{}
	elements := newFakeElementUpserter()
	assigner := &fakeAssigner{}
	const limit = 3
	orch := newTestOrchestrator(snmpClient, jobs, results, elements, assigner, limit)

	job := domain.DiscoveryJob{ID: uuid.New(), TargetIPs: targets, SNMPCommunity: "public", SNMPVersion: domain.SNMPVersionV2c}
	if err := orch.Run(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if snmpClient.maxObserved > limit {
		t.Errorf("expected concurrency to never exceed %d, observed %d", limit, snmpClient.maxObserved)
	}
	if snmpClient.maxObserved < 2 {
		t.Errorf("expected some real concurrency (>1) to have occurred, observed %d", snmpClient.maxObserved)
	}
}

func TestOrchestrator_Run_EmptyTargetsCompletesImmediately(t *testing.T) {
	snmpClient := newFakeSNMPClient()
	jobs := &fakeJobRepo{}
	results := &fakeResultRepo{}
	elements := newFakeElementUpserter()
	assigner := &fakeAssigner{}
	orch := newTestOrchestrator(snmpClient, jobs, results, elements, assigner, 5)

	job := domain.DiscoveryJob{ID: uuid.New(), TargetIPs: nil, SNMPCommunity: "public", SNMPVersion: domain.SNMPVersionV2c}
	if err := orch.Run(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jobs.statuses[len(jobs.statuses)-1] != domain.DiscoveryJobStatusCompleted {
		t.Fatalf("expected completed status, got %v", jobs.statuses)
	}
	if len(results.results) != 0 {
		t.Errorf("expected no results for empty target list, got %d", len(results.results))
	}
}
