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
	"netcluster/internal/ruleengine"
)

type fakeElementsRepo struct {
	byID         map[uuid.UUID]domain.NetworkElement
	deletedIDs   []uuid.UUID
	lastFilter   apihttp.ElementFilter
}

func newFakeElementsRepo() *fakeElementsRepo {
	return &fakeElementsRepo{byID: map[uuid.UUID]domain.NetworkElement{}}
}

func (f *fakeElementsRepo) Get(ctx context.Context, id uuid.UUID) (domain.NetworkElement, error) {
	e, ok := f.byID[id]
	if !ok {
		return domain.NetworkElement{}, errNotFound
	}
	return e, nil
}

func (f *fakeElementsRepo) List(ctx context.Context, filter apihttp.ElementFilter) ([]domain.NetworkElement, error) {
	f.lastFilter = filter
	var out []domain.NetworkElement
	for _, e := range f.byID {
		if filter.AssignmentStatus != "" && e.AssignmentStatus != filter.AssignmentStatus {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeElementsRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if _, ok := f.byID[id]; !ok {
		return errNotFound
	}
	delete(f.byID, id)
	f.deletedIDs = append(f.deletedIDs, id)
	return nil
}

type fakeElementHistoryRepo struct {
	byElement map[uuid.UUID][]domain.AssignmentHistory
}

func (f *fakeElementHistoryRepo) ListByElement(ctx context.Context, elementID uuid.UUID) ([]domain.AssignmentHistory, error) {
	return f.byElement[elementID], nil
}

type fakeElementsAssigner struct {
	calls []uuid.UUID
	err   error
}

func (f *fakeElementsAssigner) Assign(ctx context.Context, elementID uuid.UUID, hostname string, rules []domain.Rule, trigger string) (*ruleengine.MatchResult, error) {
	f.calls = append(f.calls, elementID)
	if f.err != nil {
		return nil, f.err
	}
	return &ruleengine.MatchResult{}, nil
}

type fakeElementRuleLister struct{}

func (fakeElementRuleLister) ListEnabledOrdered(ctx context.Context) ([]domain.Rule, error) {
	return []domain.Rule{{IsEnabled: true, Pattern: `^sw-`, ClusterKeyTemplate: "x"}}, nil
}

func newElementsTestServer() (*fakeElementsRepo, *fakeElementsAssigner, http.Handler) {
	elements := newFakeElementsRepo()
	history := &fakeElementHistoryRepo{byElement: map[uuid.UUID][]domain.AssignmentHistory{}}
	assigner := &fakeElementsAssigner{}
	handler := apihttp.NewElementsHandler(elements, history, fakeElementRuleLister{}, assigner)
	r := chi.NewRouter()
	r.Route("/api/v1/elements", func(r chi.Router) {
		r.Get("/", handler.List)
		r.Post("/redistribute", handler.RedistributeBulk)
		r.Get("/{id}", handler.Get)
		r.Get("/{id}/history", handler.History)
		r.Post("/{id}/redistribute", handler.RedistributeSingle)
		r.Delete("/{id}", handler.Delete)
	})
	return elements, assigner, r
}

func TestElementsHandler_List_ReturnsElements(t *testing.T) {
	elements, _, srv := newElementsTestServer()
	id := uuid.New()
	elements.byID[id] = domain.NetworkElement{ID: id, IPAddress: "10.0.0.1", SysName: "sw-dc1-01", AssignmentStatus: domain.AssignmentStatusUnassigned}

	rec := doJSON(t, srv, http.MethodGet, "/api/v1/elements/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp []dto.ElementResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(resp) != 1 || resp[0].SysName != "sw-dc1-01" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestElementsHandler_List_PassesPaginationToRepo(t *testing.T) {
	elements, _, srv := newElementsTestServer()

	rec := doJSON(t, srv, http.MethodGet, "/api/v1/elements/?limit=25&offset=50", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if elements.lastFilter.Limit != 25 || elements.lastFilter.Offset != 50 {
		t.Errorf("expected filter Limit=25 Offset=50, got %+v", elements.lastFilter)
	}
}

func TestElementsHandler_List_DefaultsPaginationWhenUnset(t *testing.T) {
	elements, _, srv := newElementsTestServer()

	rec := doJSON(t, srv, http.MethodGet, "/api/v1/elements/", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if elements.lastFilter.Limit != 100 {
		t.Errorf("expected default limit 100, got %d", elements.lastFilter.Limit)
	}
}

func TestElementsHandler_List_InvalidPaginationReturns400(t *testing.T) {
	_, _, srv := newElementsTestServer()

	rec := doJSON(t, srv, http.MethodGet, "/api/v1/elements/?limit=-5", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestElementsHandler_Get_UnknownIDReturns404(t *testing.T) {
	_, _, srv := newElementsTestServer()

	rec := doJSON(t, srv, http.MethodGet, "/api/v1/elements/"+uuid.New().String(), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestElementsHandler_History_ReturnsTimeline(t *testing.T) {
	elements := newFakeElementsRepo()
	history := &fakeElementHistoryRepo{byElement: map[uuid.UUID][]domain.AssignmentHistory{}}
	assigner := &fakeElementsAssigner{}
	handler := apihttp.NewElementsHandler(elements, history, fakeElementRuleLister{}, assigner)
	r := chi.NewRouter()
	r.Get("/api/v1/elements/{id}/history", handler.History)

	id := uuid.New()
	history.byElement[id] = []domain.AssignmentHistory{
		{ID: uuid.New(), ElementID: id, TriggerSource: domain.TriggerSourceDiscovery, RenderedClusterKey: "dc1-core"},
	}

	rec := doJSON(t, r, http.MethodGet, "/api/v1/elements/"+id.String()+"/history", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp []dto.AssignmentHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(resp) != 1 || resp[0].RenderedClusterKey != "dc1-core" {
		t.Errorf("unexpected history: %+v", resp)
	}
}

func TestElementsHandler_RedistributeSingle_CallsAssigner(t *testing.T) {
	elements, assigner, srv := newElementsTestServer()
	id := uuid.New()
	elements.byID[id] = domain.NetworkElement{ID: id, SysName: "sw-dc1-01"}

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/elements/"+id.String()+"/redistribute", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(assigner.calls) != 1 || assigner.calls[0] != id {
		t.Errorf("expected assigner called once with %v, got %v", id, assigner.calls)
	}
}

func TestElementsHandler_RedistributeSingle_UnknownIDReturns404(t *testing.T) {
	_, assigner, srv := newElementsTestServer()

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/elements/"+uuid.New().String()+"/redistribute", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(assigner.calls) != 0 {
		t.Errorf("expected assigner not called, got %d calls", len(assigner.calls))
	}
}

func TestElementsHandler_RedistributeBulk_DefaultsToAllElements(t *testing.T) {
	elements, assigner, srv := newElementsTestServer()
	id1, id2 := uuid.New(), uuid.New()
	elements.byID[id1] = domain.NetworkElement{ID: id1, SysName: "sw-a"}
	elements.byID[id2] = domain.NetworkElement{ID: id2, SysName: "sw-b"}

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/elements/redistribute", dto.BulkRedistributeRequest{})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(assigner.calls) != 2 {
		t.Errorf("expected 2 assign calls (all elements), got %d", len(assigner.calls))
	}
}

func TestElementsHandler_RedistributeBulk_ScopesToGivenIDs(t *testing.T) {
	elements, assigner, srv := newElementsTestServer()
	id1, id2 := uuid.New(), uuid.New()
	elements.byID[id1] = domain.NetworkElement{ID: id1, SysName: "sw-a"}
	elements.byID[id2] = domain.NetworkElement{ID: id2, SysName: "sw-b"}

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/elements/redistribute", dto.BulkRedistributeRequest{ElementIDs: []string{id1.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(assigner.calls) != 1 || assigner.calls[0] != id1 {
		t.Errorf("expected exactly one assign call for id1, got %v", assigner.calls)
	}
}

func TestElementsHandler_Delete_RemovesElement(t *testing.T) {
	elements, _, srv := newElementsTestServer()
	id := uuid.New()
	elements.byID[id] = domain.NetworkElement{ID: id}

	rec := doJSON(t, srv, http.MethodDelete, "/api/v1/elements/"+id.String(), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(elements.deletedIDs) != 1 || elements.deletedIDs[0] != id {
		t.Errorf("expected element soft-deleted, got %v", elements.deletedIDs)
	}
}
