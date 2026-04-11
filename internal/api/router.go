package api

import (
	"github.com/cyclist-map/cyclist-map/internal/app"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the chi router with all API routes wired up.
func NewRouter(
	routeSvc *app.RouteService,
	discoverySvc *app.DiscoveryService,
	venueSvc *app.VenueService,
	routingH *RoutingHandler,
	weatherH *WeatherHandler,
) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	rh := &RouteHandler{svc: routeSvc}
	dh := NewDiscoveryHandler(discoverySvc)
	vh := NewVenueHandler(venueSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Routes
		r.Get("/routes", rh.List)
		r.Post("/routes", rh.Create)
		r.Get("/routes/{id}", rh.Get)
		r.Patch("/routes/{id}", rh.Update)
		r.Delete("/routes/{id}", rh.Archive)

		// Discovery
		r.Get("/discover/nearby", dh.Nearby)
		r.Get("/discover/viewport", dh.Viewport)
		r.Get("/discover/suggested", dh.Suggested)

		// Venues
		r.Get("/venues/along-route", vh.AlongRoute)
		r.Get("/venues/tags", vh.Tags)

		// Routing
		r.Post("/routing/directions", routingH.Directions)

		// Weather
		r.Get("/weather/point", weatherH.AtPoint)
	})
	return r
}
