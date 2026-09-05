package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apihttp "netcluster/internal/api/http"
	"netcluster/internal/api/http/dto"
	"netcluster/internal/domain"
)

var errNotFound = domain.ErrNotFound

type fakeRuleRepo struct {
	byID          map[uuid.UUID]domain.Rule
	createCalls   int
	reorderCalls  [][]uuid.UUID
	createErr     error
}

func newFakeRuleRepo() *fakeRuleRepo {
	return &fakeRuleRepo{byID: map[uuid.UUID]domain.Rule{}}
}

func (f *fakeRuleRepo) Create(ctx context.Context, rule domain.Rule) (domain.Rule, error) {
	f.createCalls++
	if f.createErr != nil {
		return domain.Rule{}, f.createErr
	}
	rule.ID = uuid.New()
	f.byID[rule.ID] = rule
	return rule, nil
}

func (f *fakeRuleRepo) Get(ctx context.Context, id uuid.UUID) (domain.Rule, error) {
	r, ok := f.byID[id]
	if !ok {
		return domain.Rule{}, errNotFound
	}
	return r, nil
}

func (f *fakeRuleRepo) Update(ctx context.Context, rule domain.Rule) (domain.Rule, error) {
	if _, ok := f.byID[rule.ID]; !ok {
		return domain.Rule{}, errNotFound
	}
	f.byID[rule.ID] = rule
	return rule, nil
}

func (f *fakeRuleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := f.byID[id]; !ok {
		return errNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeRuleRepo) ListAllOrdered(ctx context.Context) ([]domain.Rule, error) {
	var out []domain.Rule
	for _, r := range f.byID {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeRuleRepo) Reorder(ctx context.Context, orderedIDs []uuid.UUID) error {
	f.reorderCalls = append(f.reorderCalls, orderedIDs)
	return nil
}

func (f *fakeRuleRepo) ExistsClusterByLookupKey(ctx context.Context, key string) (bool, error) {
	return false, nil
}

func newRulesTestServer() (*fakeRuleRepo, http.Handler) {
	repo := newFakeRuleRepo()
	handler := apihttp.NewRulesHandler(repo, repo)
	r := chi.NewRouter()
	r.Route("/api/v1/rules", func(r chi.Router) {
		r.Post("/", handler.Create)
		r.Get("/", handler.List)
		r.Post("/reorder", handler.Reorder)
		r.Post("/test", handler.Test)
		r.Get("/{id}", handler.Get)
		r.Put("/{id}", handler.Update)
		r.Delete("/{id}", handler.Delete)
	})
	return repo, r
}

func doJSON(t *testing.T, srv http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestRulesHandler_Create_ValidRuleReturns201(t *testing.T) {
	repo, srv := newRulesTestServer()

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/rules/", dto.RuleRequest{
		Name:               "core switches",
		Pattern:            `^sw-(?P<site>[a-z0-9]+)-core-\d+$`,
		ClusterKeyTemplate: "{site}-core",
		Priority:           10,
		IsEnabled:          true,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if repo.createCalls != 1 {
		t.Errorf("expected repo.Create called once, got %d", repo.createCalls)
	}
	var resp dto.RuleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Name != "core switches" || resp.ID == "" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestRulesHandler_Create_InvalidRegexReturns400WithFieldError(t *testing.T) {
	repo, srv := newRulesTestServer()

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/rules/", dto.RuleRequest{
		Name: "broken", Pattern: `(unterminated`, ClusterKeyTemplate: "x",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if repo.createCalls != 0 {
		t.Errorf("expected repo.Create NOT to be called for invalid rule, got %d calls", repo.createCalls)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	errObj := body["error"].(map[string]any)
	fields := errObj["fields"].([]any)
	if len(fields) == 0 {
		t.Error("expected at least one field error")
	}
}

func TestRulesHandler_Get_UnknownIDReturns404(t *testing.T) {
	_, srv := newRulesTestServer()

	rec := doJSON(t, srv, http.MethodGet, "/api/v1/rules/"+uuid.New().String(), nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRulesHandler_Delete_RemovesRule(t *testing.T) {
	repo, srv := newRulesTestServer()
	created, _ := repo.Create(context.Background(), domain.Rule{Name: "x", Pattern: "a", ClusterKeyTemplate: "x"})

	rec := doJSON(t, srv, http.MethodDelete, "/api/v1/rules/"+created.ID.String(), nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := repo.byID[created.ID]; ok {
		t.Error("expected rule to be removed from repo")
	}
}

func TestRulesHandler_Reorder_CallsRepoWithOrderedIDs(t *testing.T) {
	repo, srv := newRulesTestServer()
	idA, idB := uuid.New(), uuid.New()

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/rules/reorder", dto.ReorderRequest{RuleIDs: []uuid.UUID{idB, idA}})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(repo.reorderCalls) != 1 || len(repo.reorderCalls[0]) != 2 {
		t.Fatalf("expected one reorder call with 2 IDs, got %+v", repo.reorderCalls)
	}
	if repo.reorderCalls[0][0] != idB || repo.reorderCalls[0][1] != idA {
		t.Errorf("expected order [idB, idA], got %+v", repo.reorderCalls[0])
	}
}

func TestRulesHandler_Test_ExtractsWithoutPersisting(t *testing.T) {
	repo, srv := newRulesTestServer()

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/rules/test", dto.RuleTestRequest{
		Hostname:           "sw-dc1-core-01",
		Pattern:            `^sw-(?P<site>[a-z0-9]+)-core-\d+$`,
		ClusterKeyTemplate: "{site}-core",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if repo.createCalls != 0 {
		t.Errorf("expected rule tester never to persist, got %d create calls", repo.createCalls)
	}

	var resp dto.RuleTestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Matched || resp.RenderedClusterKey != "dc1-core" {
		t.Errorf("unexpected test response: %+v", resp)
	}
	if resp.RawCaptures["site"] != "dc1" {
		t.Errorf("expected raw capture site=dc1, got %+v", resp.RawCaptures)
	}
}

func TestRulesHandler_Test_InvalidRuleReturns400(t *testing.T) {
	_, srv := newRulesTestServer()

	rec := doJSON(t, srv, http.MethodPost, "/api/v1/rules/test", dto.RuleTestRequest{
		Hostname: "sw-dc1-01", Pattern: `(unterminated`, ClusterKeyTemplate: "x",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
