package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apihttp "netcluster/internal/api/http"
	"netcluster/internal/api/http/dto"
	"netcluster/internal/domain"
)

type fakeClusterRepo struct {
	byID      map[uuid.UUID]domain.ClusterWithCount
	deleteErr error
}

func newFakeClusterRepo() *fakeClusterRepo {
	return &fakeClusterRepo{byID: map[uuid.UUID]domain.ClusterWithCount{}}
}

func (f *fakeClusterRepo) Get(ctx context.Context, id uuid.UUID) (domain.Cluster, error) {
	c, ok := f.byID[id]
	if !ok {
		return domain.Cluster{}, errNotFound
	}
	return c.Cluster, nil
}

func (f *fakeClusterRepo) List(ctx context.Context) ([]domain.ClusterWithCount, error) {
	var out []domain.ClusterWithCount
	for _, c := range f.byID {
		out = append(out, c)
	}
	return out, nil
}

func (f *fakeClusterRepo) Rename(ctx context.Context, id uuid.UUID, displayName string, description *string) (domain.Cluster, error) {
	c, ok := f.byID[id]
	if !ok {
		return domain.Cluster{}, errNotFound
	}
	c.DisplayName = displayName
	if description != nil {
		c.Description = *description
	}
	f.byID[id] = c
	return c.Cluster, nil
}

func (f *fakeClusterRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.byID[id]; !ok {
		return errNotFound
	}
	delete(f.byID, id)
	return nil
}

func newClustersTestServer() (*fakeClusterRepo, http.Handler) {
	repo := newFakeClusterRepo()
	handler := apihttp.NewClustersHandler(repo)
	r := chi.NewRouter()
	r.Route("/api/v1/clusters", func(r chi.Router) {
		r.Get("/", handler.List)
		r.Get("/{id}", handler.Get)
		r.Patch("/{id}", handler.Rename)
		r.Delete("/{id}", handler.Delete)
	})
	return repo, r
}

func TestClustersHandler_List_ReturnsAllClusters(t *testing.T) {
	repo, srv := newClustersTestServer()
	id := uuid.New()
	repo.byID[id] = domain.ClusterWithCount{
		Cluster:     domain.Cluster{ID: id, LookupKey: "dc1-core", DisplayName: "DC1 Core"},
		MemberCount: 3,
	}

	rec := doJSON(t, srv, http.MethodGet, "/api/v1/clusters/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp []dto.ClusterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp) != 1 || resp[0].MemberCount != 3 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestClustersHandler_Get_UnknownIDReturns404(t *testing.T) {
	_, srv := newClustersTestServer()

	rec := doJSON(t, srv, http.MethodGet, "/api/v1/clusters/"+uuid.New().String(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestClustersHandler_Rename_UpdatesDisplayName(t *testing.T) {
	repo, srv := newClustersTestServer()
	id := uuid.New()
	repo.byID[id] = domain.ClusterWithCount{Cluster: domain.Cluster{ID: id, LookupKey: "dc1-core", DisplayName: "old"}}

	rec := doJSON(t, srv, http.MethodPatch, "/api/v1/clusters/"+id.String(), dto.ClusterRenameRequest{DisplayName: "DC1 Core"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if repo.byID[id].DisplayName != "DC1 Core" {
		t.Errorf("expected display name updated, got %q", repo.byID[id].DisplayName)
	}
	// Renaming must never touch lookup_key: the rule engine needs it stable
	// to keep re-finding this cluster on future matches.
	if repo.byID[id].LookupKey != "dc1-core" {
		t.Errorf("expected lookup_key to stay dc1-core, got %q", repo.byID[id].LookupKey)
	}
}

func TestClustersHandler_Delete_NonEmptyClusterReturns409(t *testing.T) {
	repo, srv := newClustersTestServer()
	id := uuid.New()
	repo.byID[id] = domain.ClusterWithCount{Cluster: domain.Cluster{ID: id}}
	repo.deleteErr = apihttp.ErrClusterNotEmpty

	rec := doJSON(t, srv, http.MethodDelete, "/api/v1/clusters/"+id.String(), nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestClustersHandler_Delete_EmptyClusterReturns204(t *testing.T) {
	repo, srv := newClustersTestServer()
	id := uuid.New()
	repo.byID[id] = domain.ClusterWithCount{Cluster: domain.Cluster{ID: id}}

	rec := doJSON(t, srv, http.MethodDelete, "/api/v1/clusters/"+id.String(), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}
