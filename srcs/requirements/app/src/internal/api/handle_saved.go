package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"CloudFGD/app/internal/store"
)

func (s *Server) handleListSaved(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	f := store.Filter{
		Source: params.Get("source"),
		Tag:    normalizeTag(params.Get("tag")),
		From:   params.Get("from"),
		To:     params.Get("to"),
	}

	if f.Source != "" {
		if _, ok := s.sourceByName(f.Source); !ok {
			writeError(w, http.StatusBadRequest, "fuente desconocida: "+f.Source)
			return
		}
	}
	for _, d := range []string{f.From, f.To} {
		if d != "" && !isoDate.MatchString(d) {
			writeError(w, http.StatusBadRequest, "las fechas deben tener el formato AAAA-MM-DD")
			return
		}
	}

	writeJSON(w, http.StatusOK, s.store.List(f))
}

func (s *Server) handleCreateSaved(w http.ResponseWriter, r *http.Request) {
	var req SaveRequest
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON no válido")
		return
	}

	item, err := s.validateSave(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	saved, err := s.store.Add(item)
	switch {
	case errors.Is(err, store.ErrDuplicate):
		writeJSON(w, http.StatusConflict, ErrorResponse{Error: err.Error(), ID: saved.ID})
	case err != nil:
		log.Println("error guardando elemento:", err)
		writeError(w, http.StatusInternalServerError, "no se pudo guardar")
	default:
		writeJSON(w, http.StatusCreated, saved)
	}
}

func (s *Server) handleDeleteSaved(w http.ResponseWriter, r *http.Request) {
	err := s.store.Delete(r.PathValue("id"))
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case err != nil:
		log.Println("error borrando elemento:", err)
		writeError(w, http.StatusInternalServerError, "no se pudo borrar")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
