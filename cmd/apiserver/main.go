package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	apihttp "netcluster/internal/api/http"
	"netcluster/internal/assignment"
	"netcluster/internal/config"
	"netcluster/internal/discovery"
	"netcluster/internal/snmp"
	"netcluster/internal/storage/postgres"
	"netcluster/web"

	_ "netcluster/docs"
)

// @title Netcluster API
// @version 1.0
// @description SNMP discovery + rule-based virtual cluster assignment for network devices (Phase 1).
// @BasePath /api/v1
func main() {
	cfg := config.Load()

	if err := postgres.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	ruleRepo := postgres.NewRuleRepo(pool)
	clusterRepo := postgres.NewClusterRepo(pool)
	elementRepo := postgres.NewElementRepo(pool)
	historyRepo := postgres.NewHistoryRepo(pool)
	discoveryJobRepo := postgres.NewDiscoveryJobRepo(pool)
	discoveryResultRepo := postgres.NewDiscoveryResultRepo(pool)

	assignSvc := assignment.NewService(elementRepo, clusterRepo, historyRepo)

	snmpClient := snmp.NewGoSNMPClient(2*time.Second, 1)
	orchestrator := discovery.NewOrchestrator(
		snmpClient, discoveryJobRepo, discoveryResultRepo, elementRepo, ruleRepo, assignSvc,
		cfg.DiscoveryWorkerPoolSize,
	)

	rulesHandler := apihttp.NewRulesHandler(ruleRepo, clusterRepo)
	clustersHandler := apihttp.NewClustersHandler(clusterRepo)
	discoveryHandler := apihttp.NewDiscoveryHandler(discoveryJobRepo, discoveryResultRepo, orchestrator)
	elementsHandler := apihttp.NewElementsHandler(elementRepo, historyRepo, ruleRepo, assignSvc)

	router := apihttp.NewRouter(rulesHandler, clustersHandler, discoveryHandler, elementsHandler, web.DistFS)

	log.Printf("listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
