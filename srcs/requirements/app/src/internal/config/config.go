package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Port            string
	DataDir         string
	SearchTimeout   time.Duration
	BOEBaseURL      string
	DatosGobBaseURL string
}

func Load() (Config, error) {
	timeout, err := time.ParseDuration(get("SEARCH_TIMEOUT", "10s"))
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("SEARCH_TIMEOUT no válido: %q", os.Getenv("SEARCH_TIMEOUT"))
	}

	return Config{
		Port:            get("APP_PORT", "8080"),
		DataDir:         get("DATA_PATH", "/data"),
		SearchTimeout:   timeout,
		BOEBaseURL:      os.Getenv("BOE_BASE_URL"),
		DatosGobBaseURL: os.Getenv("DATOSGOB_BASE_URL"),
	}, nil
}

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
