package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"CloudFGD/app/internal/sources"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()

	q := strings.TrimSpace(params.Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "falta el texto de búsqueda (q)")
		return
	}
	if utf8.RuneCountInString(q) > MaxQueryLen {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("la búsqueda no puede pasar de %d caracteres", MaxQueryLen))
		return
	}

	limit, err := parseLimit(params.Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	srcs := s.sources
	if name := params.Get("source"); name != "" && name != "all" {
		src, ok := s.sourceByName(name)
		if !ok {
			writeError(w, http.StatusBadRequest, "fuente desconocida: "+name)
			return
		}
		srcs = []sources.Source{src}
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.searchTimeout)
	defer cancel()

	results, errs := sources.SearchAll(ctx, srcs, q, limit)
	for _, e := range errs {
		log.Printf("búsqueda %q en %s falló: %s", q, e.Source, e.Message)
	}

	status := http.StatusOK
	if len(results) == 0 && len(errs) == len(srcs) {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, SearchResponse{Query: q, Results: results, Errors: errs})
}

func parseLimit(v string) (int, error) {
	if v == "" {
		return DefaultLimit, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > MaxLimit {
		return 0, fmt.Errorf("limit debe ser un número entre 1 y %d", MaxLimit)
	}
	return n, nil
}
