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
	routingIntentH *RoutingIntentHandler,
	weatherH *WeatherHandler,
	conditionsH *ConditionsHandler,
	planH *PlanHandler,
	authSvc *app.AuthService,
	communityH *CommunityHandler,
) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestSize(1 << 20))

	rh := &RouteHandler{svc: routeSvc}
	dh := NewDiscoveryHandler(discoverySvc)
	vh := NewVenueHandler(venueSvc)
	authH := NewAuthHandler(authSvc)

	r.Route("/api/v1", func(r chi.Router) {
		// Routes (read-only; seeded by migrations)
		r.Get("/routes", rh.List)
		r.Get("/routes/{id}", rh.Get)
		r.Get("/routes/{id}/conditions", conditionsH.RouteConditions)
		r.Get("/routes/{id}/reviews", communityH.ListReviews)

		// Discovery
		r.Get("/discover/nearby", dh.Nearby)
		r.Get("/discover/viewport", dh.Viewport)
		r.Get("/discover/suggested", dh.Suggested)

		// Venues
		r.Get("/venues/along-route", vh.AlongRoute)
		r.Get("/venues/tags", vh.Tags)

		// Routing
		r.Post("/routing/directions", routingH.Directions)
		// Public but spends Anthropic credits: cap concurrent calls.
		r.With(middleware.Throttle(2)).Post("/routing/intent", routingIntentH.Interpret)
		r.Post("/routing/conditions", conditionsH.RoutingConditions)

		// Weather
		r.Get("/weather/point", weatherH.AtPoint)
		r.Get("/weather/grid", weatherH.Grid)

		// Plans (public read)
		r.Get("/plans/{id}", planH.GetPlan)

		// Auth
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)

		// Community (public read)
		r.Get("/users/{id}/ride-logs", communityH.ListUserRideLogs)

		// Protected writes
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(authSvc))
			r.Post("/plans", planH.CreatePlan)
			r.Post("/plans/{id}/stops", planH.AddStop)
			r.Delete("/plans/{id}/stops/{stop_id}", planH.RemoveStop)
			r.Post("/plans/{id}/tasks", planH.AddTask)
			r.Post("/routes/{id}/plans", planH.CreatePlanFromRoute)
			r.Post("/contributions", communityH.SubmitContribution)
			r.Post("/routes/{id}/reviews", communityH.AddReview)
			r.Post("/routes/{id}/ride-logs", communityH.LogRide)
		})
	})
	return r
}
