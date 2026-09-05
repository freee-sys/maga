package config

import (
	"os"
	"strconv"
)

type Config struct {
	ListenAddr              string
	DatabaseURL             string
	DefaultSNMPCommunity    string
	DiscoveryWorkerPoolSize int
}

func Load() Config {
	return Config{
		ListenAddr:              getEnv("NETCLUSTER_LISTEN_ADDR", ":8080"),
		DatabaseURL:             getEnv("NETCLUSTER_DATABASE_URL", "postgres://postgres:postgres@localhost:55432/netcluster?sslmode=disable"),
		DefaultSNMPCommunity:    getEnv("NETCLUSTER_DEFAULT_SNMP_COMMUNITY", "public"),
		DiscoveryWorkerPoolSize: getEnvInt("NETCLUSTER_DISCOVERY_WORKER_POOL_SIZE", 20),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
