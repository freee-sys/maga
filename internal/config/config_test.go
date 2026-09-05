package config

import "testing"

func TestLoad_UsesDefaultsWhenEnvUnset(t *testing.T) {
	cfg := Load()

	if cfg.ListenAddr != ":8080" {
		t.Errorf("expected default ListenAddr :8080, got %q", cfg.ListenAddr)
	}
	if cfg.DiscoveryWorkerPoolSize != 20 {
		t.Errorf("expected default DiscoveryWorkerPoolSize 20, got %d", cfg.DiscoveryWorkerPoolSize)
	}
}

func TestLoad_ReadsOverridesFromEnv(t *testing.T) {
	t.Setenv("NETCLUSTER_LISTEN_ADDR", ":9090")
	t.Setenv("NETCLUSTER_DATABASE_URL", "postgres://user:pass@db:5432/mydb")
	t.Setenv("NETCLUSTER_DISCOVERY_WORKER_POOL_SIZE", "5")

	cfg := Load()

	if cfg.ListenAddr != ":9090" {
		t.Errorf("expected ListenAddr :9090, got %q", cfg.ListenAddr)
	}
	if cfg.DatabaseURL != "postgres://user:pass@db:5432/mydb" {
		t.Errorf("expected overridden DatabaseURL, got %q", cfg.DatabaseURL)
	}
	if cfg.DiscoveryWorkerPoolSize != 5 {
		t.Errorf("expected DiscoveryWorkerPoolSize 5, got %d", cfg.DiscoveryWorkerPoolSize)
	}
}
