package http

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"
)

// NewRouter wires the API and, if spaFS is non-nil, the built web UI —
// served for anything not matched by an API route so client-side routing
// keeps working on a full page load or refresh.
func NewRouter(rules *RulesHandler, clusters *ClustersHandler, discovery *DiscoveryHandler, elements *ElementsHandler, spaFS fs.FS) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", HealthHandler)
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Route("/api/v1/rules", func(r chi.Router) {
		r.Post("/", rules.Create)
		r.Get("/", rules.List)
		r.Post("/reorder", rules.Reorder)
		r.Post("/test", rules.Test)
		r.Get("/{id}", rules.Get)
		r.Put("/{id}", rules.Update)
		r.Delete("/{id}", rules.Delete)
	})

	r.Route("/api/v1/clusters", func(r chi.Router) {
		r.Get("/", clusters.List)
		r.Get("/{id}", clusters.Get)
		r.Patch("/{id}", clusters.Rename)
		r.Delete("/{id}", clusters.Delete)
	})

	r.Route("/api/v1/discovery/jobs", func(r chi.Router) {
		r.Post("/", discovery.Create)
		r.Get("/", discovery.List)
		r.Get("/{jobId}", discovery.Get)
		r.Get("/{jobId}/results", discovery.Results)
		r.Post("/{jobId}/cancel", discovery.Cancel)
	})

	r.Route("/api/v1/elements", func(r chi.Router) {
		r.Get("/", elements.List)
		r.Post("/redistribute", elements.RedistributeBulk)
		r.Get("/{id}", elements.Get)
		r.Get("/{id}/history", elements.History)
		r.Post("/{id}/redistribute", elements.RedistributeSingle)
		r.Delete("/{id}", elements.Delete)
	})

	if spaFS != nil {
		r.NotFound(NewSPAHandler(spaFS).ServeHTTP)
	}

	return r
}
