package api

import (
	"komorebi/internal/app"
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
	conditionsH *ConditionsHandler,
	previewH *PreviewHandler,
	planH *PlanHandler,
	authSvc *app.AuthService,
	communityH *CommunityHandler,
) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	rh := &RouteHandler{svc: routeSvc}
	dh := NewDiscoveryHandler(discoverySvc)
	vh := NewVenueHandler(venueSvc)
	authH := NewAuthHandler(authSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Routes
		r.Get("/routes", rh.List)
		r.Post("/routes", rh.Create)
		r.Get("/routes/{id}", rh.Get)
		r.Patch("/routes/{id}", rh.Update)
		r.Delete("/routes/{id}", rh.Archive)
		r.Get("/routes/{id}/conditions", conditionsH.RouteConditions)

		// Discovery
		r.Get("/discover/nearby", dh.Nearby)
		r.Get("/discover/viewport", dh.Viewport)
		r.Get("/discover/suggested", dh.Suggested)

		// Venues
		r.Get("/venues/along-route", vh.AlongRoute)
		r.Get("/venues/tags", vh.Tags)

		// Routing
		r.Post("/routing/directions", routingH.Directions)
		r.Get("/routing/conditions/preview", previewH.ConditionsPreview)

		// Weather
		r.Get("/weather/point", weatherH.AtPoint)

		// Plans
		r.Post("/plans", planH.CreatePlan)
		r.Get("/plans/{id}", planH.GetPlan)
		r.Post("/plans/{id}/stops", planH.AddStop)
		r.Delete("/plans/{id}/stops/{stop_id}", planH.RemoveStop)
		r.Post("/plans/{id}/tasks", planH.AddTask)
		r.Post("/routes/{id}/plans", planH.CreatePlanFromRoute)

		// Auth (public)
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)

		// Community — mixed auth
		// Public reads
		r.Get("/routes/{id}/reviews", communityH.ListReviews)
		r.Get("/users/{id}/ride-logs", communityH.ListUserRideLogs)

		// Protected writes
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(authSvc))
			r.Post("/contributions", communityH.SubmitContribution)
			r.Post("/routes/{id}/reviews", communityH.AddReview)
			r.Post("/routes/{id}/ride-logs", communityH.LogRide)
		})
	})
	return r
}
