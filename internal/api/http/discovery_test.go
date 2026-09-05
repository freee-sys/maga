package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apihttp "netcluster/internal/api/http"
	"netcluster/internal/api/http/dto"
	"netcluster/internal/domain"
)

type fakeDiscoveryJobRepo struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]domain.DiscoveryJob
	create int
}

func newFakeDiscoveryJobRepo() *fakeDiscoveryJobRepo {
	return &fakeDiscoveryJobRepo{byID: map[uuid.UUID]domain.DiscoveryJob{}}
}

func (f *fakeDiscoveryJobRepo) Create(ctx context.Context, job domain.DiscoveryJob) (domain.DiscoveryJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.create++
	job.ID = uuid.New()
	job.Status = domain.DiscoveryJobStatusPending
	job.TotalTargets = len(job.TargetIPs)
	f.byID[job.ID] = job
	return job, nil
}

func (f *fakeDiscoveryJobRepo) Get(ctx context.Context, id uuid.UUID) (domain.DiscoveryJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.byID[id]
	if !ok {
		return domain.DiscoveryJob{}, errNotFound
	}
	return j, nil
}

func (f *fakeDiscoveryJobRepo) List(ctx context.Context) ([]domain.DiscoveryJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.DiscoveryJob
	for _, j := range f.byID {
		out = append(out, j)
	}
	return out, nil
}

func (f *fakeDiscoveryJobRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errMsg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.byID[id]
	if !ok {
		return errNotFound
	}
	j.Status = status
	j.ErrorMessage = errMsg
	f.byID[id] = j
	return nil
}

type fakeDiscoveryResultRepo struct {
	mu      sync.Mutex
	byJobID map[uuid.UUID][]domain.DiscoveryJobResult
}

func newFakeDiscoveryResultRepo() *fakeDiscoveryResultRepo {
	return &fakeDiscoveryResultRepo{byJobID: map[uuid.UUID][]domain.DiscoveryJobResult{}}
}

func (f *fakeDiscoveryResultRepo) ListByJob(ctx context.Context, jobID uuid.UUID, limit, offset int) ([]domain.DiscoveryJobResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.byJobID[jobID], nil
}

// fakeJobRunner lets the test control exactly when/whether a background
// scan "completes," without any real SNMP or DB work. It mirrors the
// real orchestrator's contract of setting a terminal job status itself
// once its context ends, so handler-level cancel behavior can be
// observed the same way it would against the real orchestrator.
type fakeJobRunner struct {
	mu      sync.Mutex
	started []domain.DiscoveryJob
	release chan struct{}
	runErr  error
	jobs    *fakeDiscoveryJobRepo
}

func newFakeJobRunner(jobs *fakeDiscoveryJobRepo) *fakeJobRunner {
	return &fakeJobRunner{release: make(chan struct{}), jobs: jobs}
}

func (f *fakeJobRunner) Run(ctx context.Context, job domain.DiscoveryJob) error {
	f.mu.Lock()
	f.started = append(f.started, job)
	f.mu.Unlock()

	status := domain.DiscoveryJobStatusCompleted
	select {
	case <-f.release:
	case <-ctx.Done():
		status = domain.DiscoveryJobStatusCancelled
	}
	_ = f.jobs.UpdateStatus(context.Background(), job.ID, status, "")
	return f.runErr
}

func newDiscoveryTestServer() (*fakeDiscoveryJobRepo, *fakeDiscoveryResultRepo, *fakeJobRunner, http.Handler) {
	jobs := newFakeDiscoveryJobRepo()
	results := newFakeDiscoveryResultRepo()
	runner := newFakeJobRunner(jobs)
	handler := apihttp.NewDiscoveryHandler(jobs, results, runner)
	r := chi.NewRouter()
	r.Route("/api/v1/discovery/jobs", func(r chi.Router) {
		r.Post("/", handler.Create)
		r.Get("/", handler.List)
		r.Get("/{jobId}", handler.Get)
		r.Get("/{jobId}/results", handler.Results)
		r.Post("/{jobId}/cancel", handler.Cancel)
	})
	return jobs, results, runner, r
}

func TestDiscoveryHandler_Create_ValidSpecStartsJobAsync(t *testing.T) {
	jobs, _, runner, srv := newDiscoveryTestServer()
	defer close(runner.release)

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/discovery/jobs/", dto.DiscoveryJobRequest{
		Targets: "10.0.0.0/30", SNMPVersion: "v2c", SNMPCommunity: "public",
	})

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if jobs.create != 1 {
		t.Errorf("expected job created once, got %d", jobs.create)
	}

	var resp dto.DiscoveryJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.TotalTargets != 4 {
		t.Errorf("expected 4 expanded targets for a /30, got %d", resp.TotalTargets)
	}

	// The runner must actually be invoked in the background, not just accepted.
	deadline := time.After(time.Second)
	for {
		runner.mu.Lock()
		started := len(runner.started)
		runner.mu.Unlock()
		if started == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("expected background runner to be started")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func TestDiscoveryHandler_Create_InvalidTargetSpecReturns400(t *testing.T) {
	jobs, _, _, srv := newDiscoveryTestServer()

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/discovery/jobs/", dto.DiscoveryJobRequest{
		Targets: "not-a-target", SNMPVersion: "v2c", SNMPCommunity: "public",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if jobs.create != 0 {
		t.Errorf("expected no job created for an invalid spec, got %d", jobs.create)
	}
}

func TestDiscoveryHandler_Create_OversizedTargetSpecReturns400(t *testing.T) {
	_, _, _, srv := newDiscoveryTestServer()

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/discovery/jobs/", dto.DiscoveryJobRequest{
		Targets: "10.0.0.0/16", SNMPVersion: "v2c", SNMPCommunity: "public",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDiscoveryHandler_Get_UnknownIDReturns404(t *testing.T) {
	_, _, _, srv := newDiscoveryTestServer()

	rec := doJSON(t, srv, http.MethodGet, "/api/v1/discovery/jobs/"+uuid.New().String(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDiscoveryHandler_Results_ReturnsPerIPResults(t *testing.T) {
	jobs, results, _, srv := newDiscoveryTestServer()
	job, _ := jobs.Create(context.Background(), domain.DiscoveryJob{TargetIPs: []string{"10.0.0.1"}})
	results.byJobID[job.ID] = []domain.DiscoveryJobResult{
		{IPAddress: "10.0.0.1", Status: domain.DiscoveryResultStatusSuccess, SysName: "sw-dc1-01"},
	}

	rec := doJSON(t, srv, http.MethodGet, "/api/v1/discovery/jobs/"+job.ID.String()+"/results", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp []dto.DiscoveryJobResultResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 || resp[0].SysName != "sw-dc1-01" {
		t.Errorf("unexpected results: %+v", resp)
	}
}

func TestDiscoveryHandler_Cancel_StopsRunningJob(t *testing.T) {
	jobs, _, runner, srv := newDiscoveryTestServer()

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/discovery/jobs/", dto.DiscoveryJobRequest{
		Targets: "10.0.0.1", SNMPVersion: "v2c", SNMPCommunity: "public",
	})
	var created dto.DiscoveryJobResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	// Wait for the background runner to actually start before cancelling.
	deadline := time.After(time.Second)
	for {
		runner.mu.Lock()
		started := len(runner.started)
		runner.mu.Unlock()
		if started == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("runner never started")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	cancelRec := doJSON(t, srv, http.MethodPost, "/api/v1/discovery/jobs/"+created.ID+"/cancel", nil)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}

	// The runner's ctx should now be Done (the handler cancelled it),
	// which is what lets fakeJobRunner.Run return instead of blocking on
	// runner.release forever.
	id := uuid.MustParse(created.ID)
	deadline = time.After(time.Second)
	for {
		job, err := jobs.Get(context.Background(), id)
		if err == nil && job.Status != domain.DiscoveryJobStatusPending && job.Status != domain.DiscoveryJobStatusRunning {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("job never reached a terminal status after cancel, last=%+v", job)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func TestDiscoveryHandler_Cancel_UnknownJobReturns404(t *testing.T) {
	_, _, _, srv := newDiscoveryTestServer()

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/discovery/jobs/"+uuid.New().String()+"/cancel", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
