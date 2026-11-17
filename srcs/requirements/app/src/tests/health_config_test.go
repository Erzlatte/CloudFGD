package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"CloudFGD/app/internal/config"
	"CloudFGD/app/internal/health"
)

func TestHealthCheck(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ok.Close()
	ko := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ko.Close()

	if got := health.Check(ok.URL, false); got != health.Healthy {
		t.Errorf("servidor sano -> %d", got)
	}
	if got := health.Check(ko.URL, false); got != health.Unhealthy {
		t.Errorf("servidor con 503 -> %d", got)
	}
	if got := health.Check("http://127.0.0.1:1/healthz", false); got != health.Unhealthy {
		t.Errorf("servidor apagado -> %d", got)
	}
}

func TestConfigValoresPorDefecto(t *testing.T) {
	for _, k := range []string{"APP_PORT", "DATA_PATH", "SEARCH_TIMEOUT", "BOE_BASE_URL", "DATOSGOB_BASE_URL"} {
		t.Setenv(k, "")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8080" || cfg.DataDir != "/data" || cfg.SearchTimeout != 10*time.Second {
		t.Errorf("config = %+v", cfg)
	}
}

func TestConfigDesdeEntorno(t *testing.T) {
	t.Setenv("APP_PORT", "9000")
	t.Setenv("SEARCH_TIMEOUT", "3s")
	t.Setenv("BOE_BASE_URL", "http://simulado")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "9000" || cfg.SearchTimeout != 3*time.Second || cfg.BOEBaseURL != "http://simulado" {
		t.Errorf("config = %+v", cfg)
	}

	t.Setenv("SEARCH_TIMEOUT", "diez")
	if _, err := config.Load(); err == nil {
		t.Error("SEARCH_TIMEOUT no válido debería dar error")
	}
}
