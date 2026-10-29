package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"CloudFGD/app/sources"
)

const (
	maxQuery    = 100
	maxTitle    = 500
	maxSummary  = 1000
	maxNote     = 1000
	maxTags     = 10
	maxTagLen   = 30
	maxExtra    = 10
	maxExtraLen = 300
)

var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ── Fuentes y búsqueda ──

// GET /api/sources -> fuentes disponibles, para el selector del buscador.
func (a *App) listSources(w http.ResponseWriter, r *http.Request) {
	type info struct {
		Name  string `json:"name"`
		Label string `json:"label"`
	}
	out := make([]info, 0, len(a.sources))
	for _, s := range a.sources {
		out = append(out, info{s.Name(), s.Label()})
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /api/search?q=texto&source=boe|datosgob|all&limit=10
func (a *App) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "falta el texto de búsqueda (q)")
		return
	}
	if utf8.RuneCountInString(q) > maxQuery {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("la búsqueda no puede pasar de %d caracteres", maxQuery))
		return
	}

	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 25 {
			writeError(w, http.StatusBadRequest, "limit debe ser un número entre 1 y 25")
			return
		}
		limit = n
	}

	srcs := a.sources
	if name := r.URL.Query().Get("source"); name != "" && name != "all" {
		s, ok := a.sourceByName(name)
		if !ok {
			writeError(w, http.StatusBadRequest, "fuente desconocida: "+name)
			return
		}
		srcs = []sources.Source{s}
	}

	ctx, cancel := context.WithTimeout(r.Context(), a.searchTimeout)
	defer cancel()
	results, errs := sources.SearchAll(ctx, srcs, q, limit)
	for _, e := range errs {
		log.Printf("búsqueda %q en %s falló: %s", q, e.Source, e.Message)
	}

	status := http.StatusOK
	if len(results) == 0 && len(errs) == len(srcs) {
		status = http.StatusBadGateway // fallaron todas las fuentes consultadas
	}
	writeJSON(w, status, map[string]any{
		"query":   q,
		"results": results,
		"errors":  errs,
	})
}

func (a *App) sourceByName(name string) (sources.Source, bool) {
	for _, s := range a.sources {
		if s.Name() == name {
			return s, true
		}
	}
	return nil, false
}

// ── Guardados ──

// GET /api/saved?source=&tag=&from=AAAA-MM-DD&to=AAAA-MM-DD
func (a *App) listSaved(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := Filter{
		Source: q.Get("source"),
		Tag:    normalizeTag(q.Get("tag")),
		From:   q.Get("from"),
		To:     q.Get("to"),
	}
	if f.Source != "" {
		if _, ok := a.sourceByName(f.Source); !ok {
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
	writeJSON(w, http.StatusOK, a.store.List(f))
}

// savedInput es lo que envía el frontend al guardar un resultado.
type savedInput struct {
	Source     string            `json:"source"`
	ExternalID string            `json:"external_id"`
	Title      string            `json:"title"`
	Summary    string            `json:"summary"`
	Date       string            `json:"date"`
	URL        string            `json:"url"`
	Extra      map[string]string `json:"extra"`
	Tags       []string          `json:"tags"`
	Note       string            `json:"note"`
}

// POST /api/saved
func (a *App) createSaved(w http.ResponseWriter, r *http.Request) {
	var in savedInput
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "JSON no válido")
		return
	}
	item, err := a.validateSaved(in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	saved, err := a.store.Add(item)
	switch {
	case errors.Is(err, ErrDuplicate):
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "id": saved.ID})
	case err != nil:
		log.Println("error guardando elemento:", err)
		writeError(w, http.StatusInternalServerError, "no se pudo guardar")
	default:
		writeJSON(w, http.StatusCreated, saved)
	}
}

// DELETE /api/saved/{id}
func (a *App) deleteSaved(w http.ResponseWriter, r *http.Request) {
	err := a.store.Delete(r.PathValue("id"))
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case err != nil:
		log.Println("error borrando elemento:", err)
		writeError(w, http.StatusInternalServerError, "no se pudo borrar")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// validateSaved comprueba y limpia los datos antes de guardarlos.
// Los datos vienen del navegador, así que no se dan por buenos aunque procedan de una API oficial.
func (a *App) validateSaved(in savedInput) (SavedItem, error) {
	it := SavedItem{
		Source:     strings.TrimSpace(in.Source),
		ExternalID: strings.TrimSpace(in.ExternalID),
		Title:      strings.TrimSpace(in.Title),
		Summary:    strings.TrimSpace(in.Summary),
		Date:       strings.TrimSpace(in.Date),
		URL:        strings.TrimSpace(in.URL),
		Note:       strings.TrimSpace(in.Note),
	}
	if _, ok := a.sourceByName(it.Source); !ok {
		return it, errors.New("fuente desconocida")
	}
	if it.ExternalID == "" || utf8.RuneCountInString(it.ExternalID) > 200 {
		return it, errors.New("falta el identificador externo o es demasiado largo")
	}
	if it.Title == "" || utf8.RuneCountInString(it.Title) > maxTitle {
		return it, fmt.Errorf("el título es obligatorio y no puede pasar de %d caracteres", maxTitle)
	}
	if utf8.RuneCountInString(it.Summary) > maxSummary {
		return it, fmt.Errorf("el resumen no puede pasar de %d caracteres", maxSummary)
	}
	if utf8.RuneCountInString(it.Note) > maxNote {
		return it, fmt.Errorf("la nota no puede pasar de %d caracteres", maxNote)
	}
	if it.Date != "" && !isoDate.MatchString(it.Date) {
		return it, errors.New("la fecha debe tener el formato AAAA-MM-DD")
	}
	// Solo enlaces http(s): evita guardar cosas como "javascript:..." que luego se pintarían en un <a>.
	u, err := url.Parse(it.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return it, errors.New("el enlace oficial debe ser una URL http(s)")
	}

	tags, err := normalizeTags(in.Tags)
	if err != nil {
		return it, err
	}
	it.Tags = tags

	if len(in.Extra) > maxExtra {
		return it, errors.New("demasiados campos extra")
	}
	if len(in.Extra) > 0 {
		it.Extra = map[string]string{}
		for k, v := range in.Extra {
			if utf8.RuneCountInString(k) > 50 || utf8.RuneCountInString(v) > maxExtraLen {
				return it, errors.New("campo extra demasiado largo")
			}
			it.Extra[k] = v
		}
	}
	return it, nil
}

func normalizeTag(t string) string {
	return strings.Join(strings.Fields(strings.ToLower(t)), " ")
}

// normalizeTags pasa las etiquetas a minúsculas, quita vacías y repetidas.
func normalizeTags(in []string) ([]string, error) {
	out := []string{}
	for _, t := range in {
		t = normalizeTag(t)
		if t == "" || contains(out, t) {
			continue
		}
		if utf8.RuneCountInString(t) > maxTagLen {
			return nil, fmt.Errorf("cada etiqueta puede tener como mucho %d caracteres", maxTagLen)
		}
		out = append(out, t)
	}
	if len(out) > maxTags {
		return nil, fmt.Errorf("como mucho %d etiquetas", maxTags)
	}
	return out, nil
}

// ── Salud ──

// GET /healthz -> 200 si la app puede escribir en su volumen, 503 si no.
// No consulta las APIs externas: si el BOE se cae, la app sigue sana.
func (a *App) healthz(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Writable(); err != nil {
		log.Println("healthcheck fallido:", err)
		http.Error(w, "unhealthy: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok\n"))
}
