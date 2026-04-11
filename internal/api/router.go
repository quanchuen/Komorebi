package api

import (
	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the chi router with all API routes wired up.
func NewRouter(routeSvc *app.RouteService) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	rh := &RouteHandler{svc: routeSvc}
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/routes", rh.List)
		r.Post("/routes", rh.Create)
		r.Get("/routes/{id}", rh.Get)
		r.Patch("/routes/{id}", rh.Update)
		r.Delete("/routes/{id}", rh.Archive)
	})
	return r
}
