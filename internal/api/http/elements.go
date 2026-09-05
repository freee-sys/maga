package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"netcluster/internal/api/http/dto"
	"netcluster/internal/domain"
	"netcluster/internal/ruleengine"
)

// ElementFilter re-exports domain.ElementFilter for callers that only
// import this package.
type ElementFilter = domain.ElementFilter

// ElementRepo is the persistence interface the Elements handler needs.
type ElementRepo interface {
	Get(ctx context.Context, id uuid.UUID) (domain.NetworkElement, error)
	List(ctx context.Context, filter ElementFilter) ([]domain.NetworkElement, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
}

// ElementHistoryRepo is the persistence interface for an element's
// assignment audit trail.
type ElementHistoryRepo interface {
	ListByElement(ctx context.Context, elementID uuid.UUID) ([]domain.AssignmentHistory, error)
}

// ElementRuleLister loads the enabled rules a redistribute should run
// against, in priority order.
type ElementRuleLister interface {
	ListEnabledOrdered(ctx context.Context) ([]domain.Rule, error)
}

// ElementAssigner runs the rule engine and persists the outcome for one
// element. Satisfied by *assignment.Service.
type ElementAssigner interface {
	Assign(ctx context.Context, elementID uuid.UUID, hostname string, rules []domain.Rule, trigger string) (*ruleengine.MatchResult, error)
}

type ElementsHandler struct {
	elements ElementRepo
	history  ElementHistoryRepo
	rules    ElementRuleLister
	assigner ElementAssigner
}

func NewElementsHandler(elements ElementRepo, history ElementHistoryRepo, rules ElementRuleLister, assigner ElementAssigner) *ElementsHandler {
	return &ElementsHandler{elements: elements, history: history, rules: rules, assigner: assigner}
}

// List godoc
// @Summary List network elements
// @Tags elements
// @Produce json
// @Param cluster_id query string false "Filter by cluster ID"
// @Param assignment_status query string false "Filter by assignment status (unassigned|assigned)"
// @Param q query string false "Free-text search"
// @Param limit query int false "Max rows to return (default 100, max 1000)"
// @Param offset query int false "Rows to skip"
// @Success 200 {array} dto.ElementResponse
// @Router /elements [get]
func (h *ElementsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_pagination", err.Error(), nil)
		return
	}

	filter := ElementFilter{
		AssignmentStatus: r.URL.Query().Get("assignment_status"),
		Query:            r.URL.Query().Get("q"),
		Limit:            limit,
		Offset:           offset,
	}
	if raw := r.URL.Query().Get("cluster_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "cluster_id must be a UUID", nil)
			return
		}
		filter.ClusterID = &id
	}

	elements, err := h.elements.List(r.Context(), filter)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	resp := make([]dto.ElementResponse, len(elements))
	for i, e := range elements {
		resp[i] = dto.ElementFromDomain(e)
	}
	writeJSON(w, http.StatusOK, resp)
}

// Get godoc
// @Summary Get a network element by ID
// @Tags elements
// @Produce json
// @Param id path string true "Element ID"
// @Success 200 {object} dto.ElementResponse
// @Failure 404 {object} map[string]any
// @Router /elements/{id} [get]
func (h *ElementsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a UUID", nil)
		return
	}
	el, err := h.elements.Get(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		writeNotFound(w, "element not found")
		return
	}
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.ElementFromDomain(el))
}

// History godoc
// @Summary Get an element's assignment history timeline
// @Tags elements
// @Produce json
// @Param id path string true "Element ID"
// @Success 200 {array} dto.AssignmentHistoryResponse
// @Router /elements/{id}/history [get]
func (h *ElementsHandler) History(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a UUID", nil)
		return
	}
	rows, err := h.history.ListByElement(r.Context(), id)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	resp := make([]dto.AssignmentHistoryResponse, len(rows))
	for i, row := range rows {
		resp[i] = dto.AssignmentHistoryFromDomain(row)
	}
	writeJSON(w, http.StatusOK, resp)
}

// RedistributeSingle godoc
// @Summary Re-run the rule engine for one element
// @Tags elements
// @Produce json
// @Param id path string true "Element ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Router /elements/{id}/redistribute [post]
func (h *ElementsHandler) RedistributeSingle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a UUID", nil)
		return
	}

	el, err := h.elements.Get(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		writeNotFound(w, "element not found")
		return
	}
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	rules, err := h.rules.ListEnabledOrdered(r.Context())
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	result, err := h.assigner.Assign(r.Context(), el.ID, el.SysName, rules, domain.TriggerSourceManualRedistributeSingle)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rendered_cluster_key": result.RenderedClusterKey})
}

// RedistributeBulk godoc
// @Summary Re-run the rule engine for a set of elements (or all, if none given)
// @Tags elements
// @Accept json
// @Produce json
// @Param body body dto.BulkRedistributeRequest true "Optional element_ids to scope to"
// @Success 200 {object} map[string]any
// @Router /elements/redistribute [post]
func (h *ElementsHandler) RedistributeBulk(w http.ResponseWriter, r *http.Request) {
	var req dto.BulkRedistributeRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", err.Error(), nil)
			return
		}
	}

	var targets []domain.NetworkElement
	if len(req.ElementIDs) == 0 {
		all, err := h.elements.List(r.Context(), ElementFilter{})
		if err != nil {
			writeInternalError(w, err.Error())
			return
		}
		targets = all
	} else {
		for _, raw := range req.ElementIDs {
			id, err := uuid.Parse(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_id", "element_ids must all be UUIDs", nil)
				return
			}
			el, err := h.elements.Get(r.Context(), id)
			if errors.Is(err, domain.ErrNotFound) {
				writeNotFound(w, "element "+raw+" not found")
				return
			}
			if err != nil {
				writeInternalError(w, err.Error())
				return
			}
			targets = append(targets, el)
		}
	}

	rules, err := h.rules.ListEnabledOrdered(r.Context())
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	succeeded := 0
	for _, el := range targets {
		if _, err := h.assigner.Assign(r.Context(), el.ID, el.SysName, rules, domain.TriggerSourceManualRedistributeBulk); err == nil {
			succeeded++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": len(targets), "succeeded": succeeded})
}

// Delete godoc
// @Summary Soft-delete a network element
// @Tags elements
// @Param id path string true "Element ID"
// @Success 204
// @Failure 404 {object} map[string]any
// @Router /elements/{id} [delete]
func (h *ElementsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a UUID", nil)
		return
	}
	if err := h.elements.SoftDelete(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeNotFound(w, "element not found")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
