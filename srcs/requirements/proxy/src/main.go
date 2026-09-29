// Proxy inverso con TLS escrito en Go: sustituye a nginx.
// Es el único punto de entrada (puerto 443) y solo acepta TLS 1.2 y 1.3.
package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	port := env("PROXY_PORT", "443")

	// Modo healthcheck: pide /healthz a través del propio proxy (HTTPS).
	// Solo da OK si el proxy responde Y la app que hay detrás está sana.
	check := flag.Bool("healthcheck", false, "comprueba https://127.0.0.1/healthz y sale")
	flag.Parse()

	// Si se pasa --healthcheck, no arranca el proxy, solo comprueba la app.
	if *check {
		os.Exit(runHealthcheck("https://127.0.0.1:"+port+"/healthz", true))
	}

	domain := env("DOMAIN_NAME", "localhost")
	upstream, err := url.Parse(env("UPSTREAM", "http://app:8080"))
	if err != nil {
		log.Fatal(err)
	}
	certFile, keyFile, err := ensureCert(env("CERT_DIR", "/certs"), domain)
	if err != nil {
		log.Fatal(err)
	}

	// ReverseProxy reenvía cada petición a la app y añade las cabeceras
	// X-Forwarded-For / X-Forwarded-Proto para que la app sepa el origen real.
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(upstream)
			r.SetXForwarded()
			r.Out.Host = r.In.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("upstream no disponible: %v", err)
			http.Error(w, "servicio no disponible", http.StatusBadGateway)
		},
	}

	// Middleware de log: una línea por petición (equivalente al access_log de nginx).
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		proxy.ServeHTTP(w, r)
		log.Printf("%s %s %s (%s)", r.RemoteAddr, r.Method, r.URL.Path, time.Since(start))
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
		},
	}
	log.Printf("proxy TLS escuchando en %s -> %s", srv.Addr, upstream)
	log.Fatal(srv.ListenAndServeTLS(certFile, keyFile))
}
