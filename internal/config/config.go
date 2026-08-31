package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr             string
	DatabaseURL          string
	ProviderTimeout      time.Duration
	ProviderAErrorRate   float64
	ProviderATimeoutRate float64
	ProviderBErrorRate   float64
	ProviderBTimeoutRate float64
	WorkerInterval       time.Duration
	StuckAfter           time.Duration
	CatalogSeedExtra     int
}

func Load() Config {
	return Config{
		HTTPAddr:             env("HTTP_ADDR", ":8180"),
		DatabaseURL:          env("DATABASE_URL", "postgres://marketplace:marketplace@localhost:5432/marketplace?sslmode=disable"),
		ProviderTimeout:      envDuration("PROVIDER_TIMEOUT", 2*time.Second),
		ProviderAErrorRate:   envFloat("PROVIDER_A_ERROR_RATE", 0.15),
		ProviderATimeoutRate: envFloat("PROVIDER_A_TIMEOUT_RATE", 0.15),
		ProviderBErrorRate:   envFloat("PROVIDER_B_ERROR_RATE", 0.08),
		ProviderBTimeoutRate: envFloat("PROVIDER_B_TIMEOUT_RATE", 0.08),
		WorkerInterval:       envDuration("WORKER_INTERVAL", 4*time.Second),
		StuckAfter:           envDuration("STUCK_AFTER", 8*time.Second),
		CatalogSeedExtra:     envInt("CATALOG_SEED_EXTRA", 2500),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
