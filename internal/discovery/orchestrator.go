package discovery

import (
	"context"
	"errors"
	"log"
	"net"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"netcluster/internal/domain"
	"netcluster/internal/ruleengine"
	"netcluster/internal/snmp"
)

// JobRepo updates a discovery job's status as the scan progresses.
type JobRepo interface {
	UpdateStatus(ctx context.Context, jobID uuid.UUID, status string, errorMessage string) error
}

// ResultRepo records the per-IP outcome of a scan.
type ResultRepo interface {
	InsertResult(ctx context.Context, result domain.DiscoveryJobResult) error
}

// ElementUpserter creates or refreshes a network element from a
// successful SNMP response.
type ElementUpserter interface {
	UpsertDiscovered(ctx context.Context, ip string, identity snmp.Identity) (domain.NetworkElement, error)
}

// RuleLister loads the enabled rules once per job run, in priority order.
type RuleLister interface {
	ListEnabledOrdered(ctx context.Context) ([]domain.Rule, error)
}

// Assigner runs the rule engine and persists the outcome for one element.
type Assigner interface {
	Assign(ctx context.Context, elementID uuid.UUID, hostname string, rules []domain.Rule, trigger string) (*ruleengine.MatchResult, error)
}

// Orchestrator scans a discovery job's target IPs over SNMP with bounded
// concurrency. A single host's SNMP failure is recorded as a per-IP
// result and never aborts the job or its sibling scans.
type Orchestrator struct {
	snmpClient  snmp.Client
	jobs        JobRepo
	results     ResultRepo
	elements    ElementUpserter
	rules       RuleLister
	assigner    Assigner
	concurrency int
}

func NewOrchestrator(
	snmpClient snmp.Client,
	jobs JobRepo,
	results ResultRepo,
	elements ElementUpserter,
	rules RuleLister,
	assigner Assigner,
	concurrency int,
) *Orchestrator {
	return &Orchestrator{
		snmpClient:  snmpClient,
		jobs:        jobs,
		results:     results,
		elements:    elements,
		rules:       rules,
		assigner:    assigner,
		concurrency: concurrency,
	}
}

// Run scans job.TargetIPs and blocks until every target has been
// attempted. It always leaves the job in a terminal status: completed
// once all targets are attempted (regardless of how many failed), or
// failed if rules couldn't even be loaded to begin the scan.
func (o *Orchestrator) Run(ctx context.Context, job domain.DiscoveryJob) error {
	if err := o.jobs.UpdateStatus(ctx, job.ID, domain.DiscoveryJobStatusRunning, ""); err != nil {
		return err
	}

	rules, err := o.rules.ListEnabledOrdered(ctx)
	if err != nil {
		_ = o.jobs.UpdateStatus(ctx, job.ID, domain.DiscoveryJobStatusFailed, err.Error())
		return err
	}

	g := new(errgroup.Group)
	g.SetLimit(max(o.concurrency, 1))

	for _, ip := range job.TargetIPs {
		if ctx.Err() != nil {
			break // cancelled: stop launching new scans, let in-flight ones unwind
		}
		g.Go(func() error {
			o.scanOne(ctx, job, ip, rules)
			return nil // per-host failures are recorded, never propagated
		})
	}
	_ = g.Wait()

	finalStatus := domain.DiscoveryJobStatusCompleted
	if ctx.Err() != nil {
		finalStatus = domain.DiscoveryJobStatusCancelled
	}
	// A cancelled ctx can't be used for this final write — it would fail
	// against a real DB exactly like any other query would.
	return o.jobs.UpdateStatus(context.Background(), job.ID, finalStatus, "")
}

func (o *Orchestrator) scanOne(ctx context.Context, job domain.DiscoveryJob, target string, rules []domain.Rule) {
	identity, err := o.snmpClient.GetIdentity(ctx, target, job.SNMPCommunity, snmp.Version(job.SNMPVersion))

	// target may carry a port (tests point at fake agents on ephemeral
	// ports); ip_address is a real Postgres `inet` column, which can't.
	// Production targets are always bare IPs on the standard SNMP port,
	// so this is a no-op there.
	host := hostOnly(target)
	result := domain.DiscoveryJobResult{ID: uuid.New(), JobID: job.ID, IPAddress: host}

	if err != nil {
		result.ErrorMessage = err.Error()
		if errors.Is(err, snmp.ErrTimeout) {
			result.Status = domain.DiscoveryResultStatusNoResponse
		} else {
			result.Status = domain.DiscoveryResultStatusSNMPError
		}
		if err := o.results.InsertResult(ctx, result); err != nil {
			log.Printf("discovery: job %s: failed to record result for %s: %v", job.ID, host, err)
		}
		return
	}

	now := time.Now()
	result.Status = domain.DiscoveryResultStatusSuccess
	result.SysName = identity.SysName
	result.SysDescr = identity.SysDescr
	result.SysObjectID = identity.SysObjectID
	result.RespondedAt = &now

	element, err := o.elements.UpsertDiscovered(ctx, host, identity)
	if err != nil {
		result.ErrorMessage = "responded but failed to persist: " + err.Error()
		if err := o.results.InsertResult(ctx, result); err != nil {
			log.Printf("discovery: job %s: failed to record result for %s: %v", job.ID, host, err)
		}
		return
	}
	result.ElementID = &element.ID
	if err := o.results.InsertResult(ctx, result); err != nil {
		log.Printf("discovery: job %s: failed to record result for %s: %v", job.ID, host, err)
	}

	if _, err := o.assigner.Assign(ctx, element.ID, identity.SysName, rules, domain.TriggerSourceDiscovery); err != nil {
		log.Printf("discovery: job %s: failed to assign element %s (%s): %v", job.ID, element.ID, host, err)
	}
}

func hostOnly(target string) string {
	if host, _, err := net.SplitHostPort(target); err == nil {
		return host
	}
	return target
}
