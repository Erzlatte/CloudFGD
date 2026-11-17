package api

import (
	"fmt"
	"net/http"
	"os"
)

func (s *Server) handlePod(w http.ResponseWriter, r *http.Request) {
	host, err := os.Hostname()
	if err != nil {
		host = "desconocido"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "Hola desde el pod %s\n", host)
}
