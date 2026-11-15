package api

import (
	"log"
	"net/http"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Writable(); err != nil {
		log.Println("healthcheck fallido:", err)
		http.Error(w, "unhealthy: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok\n"))
}
