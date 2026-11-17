package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"CloudFGD/app/internal/api"
	"CloudFGD/app/internal/config"
	"CloudFGD/app/internal/health"
	"CloudFGD/app/internal/sources"
	"CloudFGD/app/internal/store"
	"CloudFGD/app/web"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "comprueba /healthz y sale")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	if *healthcheck {
		os.Exit(health.Check("http://127.0.0.1:"+cfg.Port+"/healthz", false))
	}

	saved, err := store.New(filepath.Join(cfg.DataDir, "saved.json"))
	if err != nil {
		log.Fatal(err)
	}

	handler := api.New(api.Deps{
		Store:         saved,
		Sources:       buildSources(cfg),
		SearchTimeout: cfg.SearchTimeout,
		Static:        web.Files(),
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("app escuchando en %s (datos en %s)", srv.Addr, cfg.DataDir)
	log.Fatal(srv.ListenAndServe())
}

func buildSources(cfg config.Config) []sources.Source {
	boe := sources.NewBOE(cfg.SearchTimeout)
	if cfg.BOEBaseURL != "" {
		boe.BaseURL = cfg.BOEBaseURL
	}

	datosgob := sources.NewDatosGob(cfg.SearchTimeout)
	if cfg.DatosGobBaseURL != "" {
		datosgob.BaseURL = cfg.DatosGobBaseURL
	}

	return []sources.Source{boe, datosgob}
}
