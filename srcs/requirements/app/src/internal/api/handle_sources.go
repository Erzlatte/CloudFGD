package api

import "net/http"

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	out := make([]SourceInfo, 0, len(s.sources))
	for _, src := range s.sources {
		out = append(out, SourceInfo{Name: src.Name(), Label: src.Label()})
	}
	writeJSON(w, http.StatusOK, out)
}
