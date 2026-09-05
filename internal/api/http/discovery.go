package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"netcluster/internal/api/http/dto"
	"netcluster/internal/discovery"
	"netcluster/internal/domain"
)

// DiscoveryJobRepo is the persistence interface the Discovery handler
// needs for job records.
type DiscoveryJobRepo interface {
	Create(ctx context.Context, job domain.DiscoveryJob) (domain.DiscoveryJob, error)
	Get(ctx context.Context, id uuid.UUID) (domain.DiscoveryJob, error)
	List(ctx context.Context) ([]domain.DiscoveryJob, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, errorMessage string) error
}

// DiscoveryResultRepo is the persistence interface for per-IP results.
type DiscoveryResultRepo interface {
	ListByJob(ctx context.Context, jobID uuid.UUID, limit, offset int) ([]domain.DiscoveryJobResult, error)
}

// JobRunner executes a discovery job. Satisfied by *discovery.Orchestrator.
type JobRunner interface {
	Run(ctx context.Context, job domain.DiscoveryJob) error
}

type DiscoveryHandler struct {
	jobs    DiscoveryJobRepo
	results DiscoveryResultRepo
	runner  JobRunner

	mu      sync.Mutex
	cancels map[uuid.UUID]context.CancelFunc
}

func NewDiscoveryHandler(jobs DiscoveryJobRepo, results DiscoveryResultRepo, runner JobRunner) *DiscoveryHandler {
	return &DiscoveryHandler{jobs: jobs, results: results, runner: runner, cancels: map[uuid.UUID]context.CancelFunc{}}
}

// Create godoc
// @Summary Start an async SNMP discovery scan
// @Tags discovery
// @Accept json
// @Produce json
// @Param body body dto.DiscoveryJobRequest true "IP pool / CIDR / range plus SNMP settings"
// @Success 202 {object} dto.DiscoveryJobResponse
// @Failure 400 {object} map[string]any
// @Router /discovery/jobs [post]
func (h *DiscoveryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.DiscoveryJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error(), nil)
		return
	}

	targets, err := discovery.ExpandTargets(req.Targets)
	if err != nil {
		writeValidationError(w, []FieldError{{Field: "targets", Message: err.Error()}})
		return
	}

	version := req.SNMPVersion
	if version == "" {
		version = domain.SNMPVersionV2c
	}
	if version != domain.SNMPVersionV1 && version != domain.SNMPVersionV2c {
		writeValidationError(w, []FieldError{{Field: "snmp_version", Message: "must be \"v1\" or \"v2c\""}})
		return
	}
	if req.SNMPCommunity == "" {
		writeValidationError(w, []FieldError{{Field: "snmp_community", Message: "must not be empty"}})
		return
	}

	job, err := h.jobs.Create(r.Context(), domain.DiscoveryJob{
		InputSpec:     req.Targets,
		TargetIPs:     targets,
		SNMPVersion:   version,
		SNMPCommunity: req.SNMPCommunity,
	})
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	go h.runInBackground(job)

	writeJSON(w, http.StatusAccepted, dto.DiscoveryJobFromDomain(job))
}

// runInBackground executes the scan on a fresh, independently cancellable
// context — never the request's context, which is cancelled the moment
// the HTTP response is written.
func (h *DiscoveryHandler) runInBackground(job domain.DiscoveryJob) {
	ctx, cancel := context.WithCancel(context.Background())
	h.mu.Lock()
	h.cancels[job.ID] = cancel
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.cancels, job.ID)
		h.mu.Unlock()
		cancel()
	}()

	if err := h.runner.Run(ctx, job); err != nil {
		log.Printf("discovery: job %s run failed: %v", job.ID, err)
	}
}

// List godoc
// @Summary List discovery jobs
// @Tags discovery
// @Produce json
// @Success 200 {array} dto.DiscoveryJobResponse
// @Router /discovery/jobs [get]
func (h *DiscoveryHandler) List(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.jobs.List(r.Context())
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	resp := make([]dto.DiscoveryJobResponse, len(jobs))
	for i, j := range jobs {
		resp[i] = dto.DiscoveryJobFromDomain(j)
	}
	writeJSON(w, http.StatusOK, resp)
}

// Get godoc
// @Summary Get a discovery job's status
// @Tags discovery
// @Produce json
// @Param jobId path string true "Job ID"
// @Success 200 {object} dto.DiscoveryJobResponse
// @Failure 404 {object} map[string]any
// @Router /discovery/jobs/{jobId} [get]
func (h *DiscoveryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "jobId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "jobId must be a UUID", nil)
		return
	}
	job, err := h.jobs.Get(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		writeNotFound(w, "discovery job not found")
		return
	}
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.DiscoveryJobFromDomain(job))
}

// Results godoc
// @Summary List a discovery job's per-IP results
// @Tags discovery
// @Produce json
// @Param jobId path string true "Job ID"
// @Param limit query int false "Max rows to return (default 100, max 1000)"
// @Param offset query int false "Rows to skip"
// @Success 200 {array} dto.DiscoveryJobResultResponse
// @Router /discovery/jobs/{jobId}/results [get]
func (h *DiscoveryHandler) Results(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "jobId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "jobId must be a UUID", nil)
		return
	}
	limit, offset, err := parsePagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_pagination", err.Error(), nil)
		return
	}
	results, err := h.results.ListByJob(r.Context(), id, limit, offset)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	resp := make([]dto.DiscoveryJobResultResponse, len(results))
	for i, r := range results {
		resp[i] = dto.DiscoveryJobResultFromDomain(r)
	}
	writeJSON(w, http.StatusOK, resp)
}

// Cancel godoc
// @Summary Best-effort cancel a running discovery job
// @Tags discovery
// @Produce json
// @Param jobId path string true "Job ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]any
// @Router /discovery/jobs/{jobId}/cancel [post]
func (h *DiscoveryHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "jobId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "jobId must be a UUID", nil)
		return
	}

	if _, err := h.jobs.Get(r.Context(), id); errors.Is(err, domain.ErrNotFound) {
		writeNotFound(w, "discovery job not found")
		return
	}

	h.mu.Lock()
	cancel, running := h.cancels[id]
	h.mu.Unlock()

	if !running {
		writeJSON(w, http.StatusOK, map[string]string{"status": "not_running"})
		return
	}

	cancel()
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelling"})
}
