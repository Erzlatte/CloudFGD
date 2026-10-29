// App de CloudFGD: sirve el frontend y la API REST propia, que consulta
// APIs públicas (BOE, datos.gob.es) y guarda los elementos de interés.
package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"CloudFGD/app/sources"
)

// El frontend se incrusta dentro del binario en tiempo de compilación.
//
//go:embed web
var webFS embed.FS

// App agrupa las dependencias que usan los handlers.
type App struct {
	store         *Store
	sources       []sources.Source
	searchTimeout time.Duration
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (a *App) routes() http.Handler {
	web, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	static := http.FileServer(http.FS(web))

	mux := http.NewServeMux()

	// Frontend
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, web, "html/index.html")
	})
	mux.Handle("GET /css/", static)
	mux.Handle("GET /js/", static)

	// API REST
	mux.HandleFunc("GET /api/sources", a.listSources)
	mux.HandleFunc("GET /api/search", a.search)
	mux.HandleFunc("GET /api/saved", a.listSaved)
	mux.HandleFunc("POST /api/saved", a.createSaved)
	mux.HandleFunc("DELETE /api/saved/{id}", a.deleteSaved)

	// Salud del servicio (lo usa el HEALTHCHECK de Docker)
	mux.HandleFunc("GET /healthz", a.healthz)

	return mux
}

func main() {
	port := env("APP_PORT", "8080")

	check := flag.Bool("healthcheck", false, "comprueba /healthz y sale")
	flag.Parse()
	if *check {
		os.Exit(runHealthcheck("http://127.0.0.1:"+port+"/healthz", false))
	}

	timeout, err := time.ParseDuration(env("SEARCH_TIMEOUT", "10s"))
	if err != nil {
		log.Fatalf("SEARCH_TIMEOUT no válido: %v", err)
	}

	dataDir := env("DATA_PATH", "/data")
	store, err := NewStore(filepath.Join(dataDir, "saved.json"))
	if err != nil {
		log.Fatal(err)
	}

	// Para añadir una fuente nueva basta con crear su conector y añadirlo aquí.
	boe := sources.NewBOE(timeout)
	datosgob := sources.NewDatosGob(timeout)
	// Opcional: apuntar a otra URL (por ejemplo, una API simulada para pruebas).
	boe.BaseURL = env("BOE_BASE_URL", boe.BaseURL)
	datosgob.BaseURL = env("DATOSGOB_BASE_URL", datosgob.BaseURL)

	app := &App{
		store:         store,
		searchTimeout: timeout,
		sources:       []sources.Source{boe, datosgob},
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("app escuchando en %s (datos en %s)", srv.Addr, dataDir)
	log.Fatal(srv.ListenAndServe())
}
