package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"komorebi/internal/api"
	"komorebi/internal/app"
	"komorebi/internal/infra/postgres"
	"komorebi/internal/infra/valhalla"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
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
	routeRepo := postgres.NewRouteRepo(pool)
	routeSvc := app.NewRouteService(routeRepo)

	discoveryRepo := postgres.NewDiscoveryRepo(pool)
	discoverySvc := app.NewDiscoveryService(discoveryRepo)

	venueRepo := postgres.NewVenueRepo(pool)
	venueSvc := app.NewVenueService(venueRepo)

	weatherRepo := postgres.NewWeatherRepo(pool)
	weatherSvc := app.NewWeatherService(weatherRepo)
	weatherHandler := api.NewWeatherHandler(weatherSvc)

	envRepo := postgres.NewEnvironmentRepo(pool)
	envSvc := app.NewEnvironmentService(envRepo)
	conditionsHandler := api.NewConditionsHandler(routeRepo, envSvc)
	previewHandler := api.NewPreviewHandler(envRepo)

	valhallaURL := os.Getenv("VALHALLA_URL")
	if valhallaURL == "" {
		valhallaURL = "http://localhost:8002"
	}
	valhallaClient := valhalla.NewClient(valhallaURL)
	routingSvc := app.NewRoutingService(valhallaClient)
	routingHandler := api.NewRoutingHandler(routingSvc)

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

	router := api.NewRouter(routeSvc, discoverySvc, venueSvc, routingHandler, weatherHandler, conditionsHandler, previewHandler, planHandler, authSvc, communityHandler)

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
		log.Printf("cyclist-map API listening on :%s", port)
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
