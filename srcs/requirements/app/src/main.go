// App del libro de visitas: sirve el frontend (html/css/js) y una API JSON.
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
)

// El frontend se incrusta dentro del binario en tiempo de compilación,
// así la imagen final solo necesita el ejecutable.
//
//go:embed web
var webFS embed.FS

// App agrupa las dependencias que usan los handlers.
type App struct {
	store *Store
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// routes define todas las rutas HTTP de la aplicación.
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
	mux.Handle("GET /css/", static) // /css/style.css -> web/css/style.css
	mux.Handle("GET /js/", static)  // /js/app.js     -> web/js/app.js

	// API
	mux.HandleFunc("GET /api/messages", a.listMessages)
	mux.HandleFunc("POST /api/messages", a.createMessageJSON)
	mux.HandleFunc("POST /messages", a.createMessageForm)

	// Salud del servicio (lo usa el HEALTHCHECK de Docker)
	mux.HandleFunc("GET /healthz", a.healthz)

	return mux
}

func main() {
	port := env("APP_PORT", "8080")

	// Modo healthcheck: el propio binario hace la comprobación y termina
	// con código 0 (sano) o 1 (no sano). Docker lo ejecuta dentro del contenedor.
	check := flag.Bool("healthcheck", false, "comprueba /healthz y sale")
	flag.Parse()
	if *check {
		os.Exit(runHealthcheck("http://127.0.0.1:"+port+"/healthz", false))
	}

	dataDir := env("DATA_PATH", "/data")
	store, err := NewStore(filepath.Join(dataDir, "messages.json"))
	if err != nil {
		log.Fatal(err)
	}
	app := &App{store: store}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("app escuchando en %s (datos en %s)", srv.Addr, dataDir)
	log.Fatal(srv.ListenAndServe())
}
