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
)

// ErrClusterNotEmpty re-exports domain.ErrClusterNotEmpty for callers
// that only import this package.
var ErrClusterNotEmpty = domain.ErrClusterNotEmpty

// ClusterRepo is the persistence interface the Clusters handler needs.
type ClusterRepo interface {
	Get(ctx context.Context, id uuid.UUID) (domain.Cluster, error)
	List(ctx context.Context) ([]domain.ClusterWithCount, error)
	Rename(ctx context.Context, id uuid.UUID, displayName string, description *string) (domain.Cluster, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ClustersHandler struct {
	clusters ClusterRepo
}

func NewClustersHandler(clusters ClusterRepo) *ClustersHandler {
	return &ClustersHandler{clusters: clusters}
}

// List godoc
// @Summary List clusters with member counts
// @Tags clusters
// @Produce json
// @Success 200 {array} dto.ClusterResponse
// @Router /clusters [get]
func (h *ClustersHandler) List(w http.ResponseWriter, r *http.Request) {
	clusters, err := h.clusters.List(r.Context())
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	resp := make([]dto.ClusterResponse, len(clusters))
	for i, c := range clusters {
		resp[i] = dto.ClusterFromDomain(c)
	}
	writeJSON(w, http.StatusOK, resp)
}

// Get godoc
// @Summary Get a cluster by ID
// @Tags clusters
// @Produce json
// @Param id path string true "Cluster ID"
// @Success 200 {object} dto.ClusterResponse
// @Failure 404 {object} map[string]any
// @Router /clusters/{id} [get]
func (h *ClustersHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a UUID", nil)
		return
	}
	c, err := h.clusters.Get(r.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		writeNotFound(w, "cluster not found")
		return
	}
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.ClusterFromDomain(domain.ClusterWithCount{Cluster: c}))
}

// Rename godoc
// @Summary Rename a cluster's display name/description
// @Description Renaming never touches lookup_key, so the rule engine keeps re-finding this cluster on future matches.
// @Tags clusters
// @Accept json
// @Produce json
// @Param id path string true "Cluster ID"
// @Param body body dto.ClusterRenameRequest true "New display name/description"
// @Success 200 {object} dto.ClusterResponse
// @Failure 404 {object} map[string]any
// @Router /clusters/{id} [patch]
func (h *ClustersHandler) Rename(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a UUID", nil)
		return
	}

	var req dto.ClusterRenameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error(), nil)
		return
	}

	c, err := h.clusters.Rename(r.Context(), id, req.DisplayName, req.Description)
	if errors.Is(err, domain.ErrNotFound) {
		writeNotFound(w, "cluster not found")
		return
	}
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.ClusterFromDomain(domain.ClusterWithCount{Cluster: c}))
}

// Delete godoc
// @Summary Delete an empty cluster
// @Tags clusters
// @Param id path string true "Cluster ID"
// @Success 204
// @Failure 404 {object} map[string]any
// @Failure 409 {object} map[string]any "cluster still has member elements"
// @Router /clusters/{id} [delete]
func (h *ClustersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a UUID", nil)
		return
	}

	err = h.clusters.Delete(r.Context(), id)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, domain.ErrNotFound):
		writeNotFound(w, "cluster not found")
	case errors.Is(err, ErrClusterNotEmpty):
		writeError(w, http.StatusConflict, "cluster_not_empty", "cluster still has member elements", nil)
	default:
		writeInternalError(w, err.Error())
	}
}
