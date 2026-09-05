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

// RuleRepo is the persistence interface the Rules handler needs.
type RuleRepo interface {
	Create(ctx context.Context, rule domain.Rule) (domain.Rule, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Rule, error)
	Update(ctx context.Context, rule domain.Rule) (domain.Rule, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListAllOrdered(ctx context.Context) ([]domain.Rule, error)
	Reorder(ctx context.Context, orderedIDs []uuid.UUID) error
}

// ClusterExistsChecker is the read-only check the rule tester uses to
// report whether its candidate rule would create a brand-new cluster,
// without ever writing one.
type ClusterExistsChecker interface {
	ExistsClusterByLookupKey(ctx context.Context, lookupKey string) (bool, error)
}

type RulesHandler struct {
	rules    RuleRepo
	clusters ClusterExistsChecker
}

func NewRulesHandler(rules RuleRepo, clusters ClusterExistsChecker) *RulesHandler {
	return &RulesHandler{rules: rules, clusters: clusters}
}

func fieldErrorsFromValidation(errs []ruleengine.FieldError) []FieldError {
	out := make([]FieldError, len(errs))
	for i, e := range errs {
		out[i] = FieldError{Field: e.Field, Message: e.Message}
	}
	return out
}

// Create godoc
// @Summary Create a rule
// @Tags rules
// @Accept json
// @Produce json
// @Param rule body dto.RuleRequest true "Rule definition"
// @Success 201 {object} dto.RuleResponse
// @Failure 400 {object} map[string]any
// @Router /rules [post]
func (h *RulesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error(), nil)
		return
	}

	if errs := ruleengine.ValidateRule(req.Pattern, req.CaptureTransforms, req.ClusterKeyTemplate); len(errs) > 0 {
		writeValidationError(w, fieldErrorsFromValidation(errs))
		return
	}

	created, err := h.rules.Create(r.Context(), req.ToDomain())
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, dto.RuleFromDomain(created))
}

// List godoc
// @Summary List rules ordered by priority
// @Tags rules
// @Produce json
// @Success 200 {array} dto.RuleResponse
// @Router /rules [get]
func (h *RulesHandler) List(w http.ResponseWriter, r *http.Request) {
	rules, err := h.rules.ListAllOrdered(r.Context())
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	resp := make([]dto.RuleResponse, len(rules))
	for i, rule := range rules {
		resp[i] = dto.RuleFromDomain(rule)
	}
	writeJSON(w, http.StatusOK, resp)
}

// Get godoc
// @Summary Get a rule by ID
// @Tags rules
// @Produce json
// @Param id path string true "Rule ID"
// @Success 200 {object} dto.RuleResponse
// @Failure 404 {object} map[string]any
// @Router /rules/{id} [get]
func (h *RulesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a UUID", nil)
		return
	}
	rule, err := h.rules.Get(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		writeNotFound(w, "rule not found")
		return
	}
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.RuleFromDomain(rule))
}

// Update godoc
// @Summary Update a rule
// @Tags rules
// @Accept json
// @Produce json
// @Param id path string true "Rule ID"
// @Param rule body dto.RuleRequest true "Rule definition"
// @Success 200 {object} dto.RuleResponse
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Router /rules/{id} [put]
func (h *RulesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a UUID", nil)
		return
	}

	var req dto.RuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error(), nil)
		return
	}
	if errs := ruleengine.ValidateRule(req.Pattern, req.CaptureTransforms, req.ClusterKeyTemplate); len(errs) > 0 {
		writeValidationError(w, fieldErrorsFromValidation(errs))
		return
	}

	rule := req.ToDomain()
	rule.ID = id
	updated, err := h.rules.Update(r.Context(), rule)
	if errors.Is(err, domain.ErrNotFound) {
		writeNotFound(w, "rule not found")
		return
	}
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.RuleFromDomain(updated))
}

// Delete godoc
// @Summary Delete a rule
// @Tags rules
// @Param id path string true "Rule ID"
// @Success 204
// @Failure 404 {object} map[string]any
// @Router /rules/{id} [delete]
func (h *RulesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a UUID", nil)
		return
	}
	if err := h.rules.Delete(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeNotFound(w, "rule not found")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reorder godoc
// @Summary Reorder rule priorities
// @Tags rules
// @Accept json
// @Produce json
// @Param body body dto.ReorderRequest true "Ordered rule IDs"
// @Success 200 {object} map[string]string
// @Router /rules/reorder [post]
func (h *RulesHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	var req dto.ReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error(), nil)
		return
	}
	if err := h.rules.Reorder(r.Context(), req.RuleIDs); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Test godoc
// @Summary Dry-run a candidate rule against a sample hostname
// @Description Evaluates a candidate rule (not necessarily persisted) against a sample hostname and reports what would happen, without writing anything: no element, no cluster, no history row.
// @Tags rules
// @Accept json
// @Produce json
// @Param body body dto.RuleTestRequest true "Candidate rule + sample hostname"
// @Success 200 {object} dto.RuleTestResponse
// @Failure 400 {object} map[string]any
// @Router /rules/test [post]
func (h *RulesHandler) Test(w http.ResponseWriter, r *http.Request) {
	var req dto.RuleTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error(), nil)
		return
	}

	if errs := ruleengine.ValidateRule(req.Pattern, req.CaptureTransforms, req.ClusterKeyTemplate); len(errs) > 0 {
		writeValidationError(w, fieldErrorsFromValidation(errs))
		return
	}

	candidate := domain.Rule{
		IsEnabled:          true,
		Pattern:            req.Pattern,
		CaptureTransforms:  req.CaptureTransforms,
		ClusterKeyTemplate: req.ClusterKeyTemplate,
	}
	result, err := ruleengine.Evaluate(req.Hostname, []domain.Rule{candidate})
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	resp := dto.RuleTestResponse{
		Matched:             result.RuleID != nil,
		RawCaptures:         result.RawCaptures,
		TransformedCaptures: result.TransformedCaptures,
		RenderedClusterKey:  result.RenderedClusterKey,
		Warnings:            result.Warnings,
	}
	if result.RenderedClusterKey != "" {
		exists, err := h.clusters.ExistsClusterByLookupKey(r.Context(), result.RenderedClusterKey)
		if err != nil {
			writeInternalError(w, err.Error())
			return
		}
		resp.WouldCreateNewCluster = !exists
	}

	writeJSON(w, http.StatusOK, resp)
}
