package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"komorebi/internal/api"
	"komorebi/internal/app"
	"komorebi/internal/infra/anthropic"
	"komorebi/internal/infra/postgres"
	"komorebi/internal/infra/valhalla"
	"komorebi/internal/infra/weatherprovider"
)

func main() {
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintln(out, "Komorebi HTTP API server.")
		fmt.Fprintln(out, "Configuration is supplied through environment variables:")
		fmt.Fprintln(out, "  DATABASE_URL      PostgreSQL/PostGIS connection string (required)")
		fmt.Fprintln(out, "  JWT_SECRET        JWT signing secret (required)")
		fmt.Fprintln(out, "  PORT              HTTP listen port (default: 8080)")
		fmt.Fprintln(out, "  VALHALLA_URL      Routing service URL (default: http://localhost:8002)")
		fmt.Fprintln(out, "  WEATHER_PROVIDER  open-meteo, tomorrow-io, or openweathermap")
		fmt.Fprintln(out, "  WEATHER_API_KEY   Required by paid weather providers")
		fmt.Fprintln(out, "  ANTHROPIC_API_KEY Enables natural-language route intent (optional)")
		fmt.Fprintln(out, "  INTENT_MODEL      Claude model for route intent (default: claude-opus-5)")
		fmt.Fprintln(out, "\nOptions:")
		flag.PrintDefaults()
	}
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Connect to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := pgxpool.New(ctx, databaseURL)
	cancel()
	if err != nil {
		log.Fatalf("failed to create connection pool: %v", err)
	}
	defer pool.Close()

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := pool.Ping(pingCtx); err != nil {
		pingCancel()
		log.Fatalf("failed to ping database: %v", err)
	}
	pingCancel()
	log.Println("connected to database")

	// Wire up dependencies
	valhallaURL := os.Getenv("VALHALLA_URL")
	if valhallaURL == "" {
		valhallaURL = "http://localhost:8002"
	}
	valhallaClient := valhalla.NewClient(valhallaURL)

	routeRepo := postgres.NewRouteRepo(pool)
	routeSvc := app.NewRouteService(routeRepo, valhallaClient)

	discoveryRepo := postgres.NewDiscoveryRepo(pool)
	discoverySvc := app.NewDiscoveryService(discoveryRepo)

	venueRepo := postgres.NewVenueRepo(pool)
	venueSvc := app.NewVenueService(venueRepo)

	weatherRepo := postgres.NewWeatherRepo(pool)
	weatherFetcher, err := weatherprovider.FromEnv()
	if err != nil {
		log.Fatalf("weather provider: %v", err)
	}
	weatherSvc := app.NewWeatherService(weatherRepo, weatherFetcher)
	weatherHandler := api.NewWeatherHandler(weatherSvc)

	envRepo := postgres.NewEnvironmentRepo(pool)
	envSvc := app.NewEnvironmentService(app.NewEnvironmentQuerierWithLiveWeather(envRepo, weatherSvc))
	conditionsHandler := api.NewConditionsHandler(routeRepo, envSvc)
	previewHandler := api.NewPreviewHandler(envRepo)

	routingSvc := app.NewRoutingService(valhallaClient)
	routingHandler := api.NewRoutingHandler(routingSvc)

	// Natural-language route intent (ADR 0003). Optional: without an API key
	// the endpoint reports 503 and structured routing is unaffected.
	var intentAdapter app.IntentAdapter
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		intentAdapter = anthropic.NewIntentAdapter(key, os.Getenv("INTENT_MODEL"))
		log.Println("route intent adapter enabled")
	} else {
		log.Println("ANTHROPIC_API_KEY not set; POST /routing/intent will return 503")
	}
	routingIntentHandler := api.NewRoutingIntentHandler(app.NewRouteIntentService(intentAdapter))

	// Plan dependencies
	planRepo := postgres.NewPlanRepo(pool)
	venueResolutionSvc := app.NewVenueResolutionService(venueRepo, venueRepo)
	planSvc := app.NewPlanService(planRepo, routeRepo, routingSvc, venueResolutionSvc)
	planHandler := api.NewPlanHandler(planSvc)

	// Auth + Community dependencies
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	userRepo := postgres.NewUserRepo(pool)
	authSvc, err := app.NewAuthService(userRepo, jwtSecret, 15*time.Minute, 7*24*time.Hour)
	if err != nil {
		log.Fatalf("failed to create auth service: %v", err)
	}

	contribRepo := postgres.NewContributionRepo(pool)
	reviewRepo := postgres.NewReviewRepo(pool)
	rideLogRepo := postgres.NewRideLogRepo(pool)
	communitySvc := app.NewCommunityService(contribRepo, reviewRepo, rideLogRepo)
	communityHandler := api.NewCommunityHandler(communitySvc)

	router := api.NewRouter(routeSvc, discoverySvc, venueSvc, routingHandler, routingIntentHandler, weatherHandler, conditionsHandler, previewHandler, planHandler, authSvc, communityHandler)

	// Start HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("komorebi API listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-stop
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("server stopped")
}
