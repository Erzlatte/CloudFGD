package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"CloudFGD/app/sources"
)

type stubSource struct {
	name    string
	results []sources.Result
	err     error
}

func (s stubSource) Name() string  { return s.name }
func (s stubSource) Label() string { return "Fuente " + s.name }
func (s stubSource) Search(ctx context.Context, q string, limit int) ([]sources.Result, error) {
	return s.results, s.err
}

func newTestApp(t *testing.T, srcs ...sources.Source) (*App, http.Handler) {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "saved.json"))
	if err != nil {
		t.Fatal(err)
	}
	app := &App{store: store, sources: srcs, searchTimeout: time.Second}
	return app, app.routes()
}

func do(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSearchAllSourcesWithPartialError(t *testing.T) {
	boe := stubSource{name: "boe", results: []sources.Result{{Source: "boe", ExternalID: "BOE-A-1", Title: "Ley"}}}
	broken := stubSource{name: "datosgob", err: sources.ErrUpstream}
	_, h := newTestApp(t, boe, broken)

	rec := do(t, h, "GET", "/api/search?q=ley", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		Results []sources.Result      `json:"results"`
		Errors  []sources.SourceError `json:"errors"`
	}
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body.Results) != 1 || len(body.Errors) != 1 || body.Errors[0].Source != "datosgob" {
		t.Errorf("respuesta = %+v", body)
	}
}

func TestSearchValidation(t *testing.T) {
	_, h := newTestApp(t, stubSource{name: "boe"})
	cases := map[string]int{
		"/api/search":                               http.StatusBadRequest, // sin q
		"/api/search?q=ley&source=nope":             http.StatusBadRequest,
		"/api/search?q=ley&limit=0":                 http.StatusBadRequest,
		"/api/search?q=ley&limit=abc":               http.StatusBadRequest,
		"/api/search?q=" + strings.Repeat("a", 101): http.StatusBadRequest,
		"/api/search?q=ley&source=boe":              http.StatusOK,
	}
	for target, want := range cases {
		if rec := do(t, h, "GET", target, ""); rec.Code != want {
			t.Errorf("%s -> %d, quiero %d", target, rec.Code, want)
		}
	}
}

func TestSearchAllFailedIs502(t *testing.T) {
	_, h := newTestApp(t, stubSource{name: "boe", err: sources.ErrUpstream})
	if rec := do(t, h, "GET", "/api/search?q=ley", ""); rec.Code != http.StatusBadGateway {
		t.Errorf("estado = %d, quiero 502", rec.Code)
	}
}

const validItem = `{"source":"boe","external_id":"BOE-A-2007-13409","title":"Ley 20/2007",
	"date":"2007-07-12","url":"https://www.boe.es/buscar/act.php?id=BOE-A-2007-13409",
	"tags":[" Autónomos ","trabajo","autónomos"],"note":"Para el TFG"}`

func TestSavedLifecycle(t *testing.T) {
	app, h := newTestApp(t, stubSource{name: "boe"}, stubSource{name: "datosgob"})

	// Crear
	rec := do(t, h, "POST", "/api/saved", validItem)
	if rec.Code != http.StatusCreated {
		t.Fatalf("crear: %d %s", rec.Code, rec.Body)
	}
	var created SavedItem
	json.NewDecoder(rec.Body).Decode(&created)
	if created.ID == "" || created.SavedAt.IsZero() {
		t.Errorf("faltan id o fecha de guardado: %+v", created)
	}
	if strings.Join(created.Tags, ",") != "autónomos,trabajo" {
		t.Errorf("etiquetas normalizadas = %v", created.Tags)
	}

	// Duplicado
	if rec := do(t, h, "POST", "/api/saved", validItem); rec.Code != http.StatusConflict {
		t.Errorf("duplicado: %d, quiero 409", rec.Code)
	}

	// Otro elemento de otra fuente y sin fecha
	other := `{"source":"datosgob","external_id":"x","title":"Paro","url":"https://datos.gob.es/catalogo/x","tags":["empleo"]}`
	if rec := do(t, h, "POST", "/api/saved", other); rec.Code != http.StatusCreated {
		t.Fatalf("crear otro: %d %s", rec.Code, rec.Body)
	}

	// Filtros
	count := func(target string) int {
		rec := do(t, h, "GET", target, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s -> %d %s", target, rec.Code, rec.Body)
		}
		var list []SavedItem
		json.NewDecoder(rec.Body).Decode(&list)
		return len(list)
	}
	checks := map[string]int{
		"/api/saved":                               2,
		"/api/saved?source=boe":                    1,
		"/api/saved?tag=EMPLEO":                    1,
		"/api/saved?tag=autónomos":                 1,
		"/api/saved?from=2007-01-01&to=2007-12-31": 1,
		"/api/saved?from=2008-01-01":               0,
	}
	for target, want := range checks {
		if got := count(target); got != want {
			t.Errorf("%s -> %d elementos, quiero %d", target, got, want)
		}
	}

	// Se conserva tras "reiniciar" (volver a cargar el fichero)
	reloaded, err := NewStore(app.store.path)
	if err != nil || len(reloaded.List(Filter{})) != 2 {
		t.Fatalf("tras recargar: %v, %d elementos", err, len(reloaded.List(Filter{})))
	}

	// Borrar
	if rec := do(t, h, "DELETE", "/api/saved/"+created.ID, ""); rec.Code != http.StatusNoContent {
		t.Errorf("borrar: %d", rec.Code)
	}
	if rec := do(t, h, "DELETE", "/api/saved/"+created.ID, ""); rec.Code != http.StatusNotFound {
		t.Errorf("borrar dos veces: %d, quiero 404", rec.Code)
	}
	if got := count("/api/saved"); got != 1 {
		t.Errorf("tras borrar quedan %d", got)
	}
}

func TestSavedValidation(t *testing.T) {
	_, h := newTestApp(t, stubSource{name: "boe"})
	bad := []string{
		`no es json`,
		`{"source":"otra","external_id":"1","title":"t","url":"https://a.es"}`,
		`{"source":"boe","external_id":"","title":"t","url":"https://a.es"}`,
		`{"source":"boe","external_id":"1","title":"","url":"https://a.es"}`,
		`{"source":"boe","external_id":"1","title":"t","url":"javascript:alert(1)"}`,
		`{"source":"boe","external_id":"1","title":"t","url":"https://a.es","date":"12/07/2007"}`,
		`{"source":"boe","external_id":"1","title":"t","url":"https://a.es","tags":["` + strings.Repeat("x", 31) + `"]}`,
		`{"source":"boe","external_id":"1","title":"t","url":"https://a.es","note":"` + strings.Repeat("n", 1001) + `"}`,
	}
	for _, body := range bad {
		if rec := do(t, h, "POST", "/api/saved", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%.60s… -> %d, quiero 400", body, rec.Code)
		}
	}
	if rec := do(t, h, "GET", "/api/saved?from=ayer", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("fecha de filtro no válida -> %d", rec.Code)
	}
}

func TestSourcesAndFrontend(t *testing.T) {
	_, h := newTestApp(t, stubSource{name: "boe"}, stubSource{name: "datosgob"})
	rec := do(t, h, "GET", "/api/sources", "")
	if !strings.Contains(rec.Body.String(), `"name":"datosgob"`) {
		t.Errorf("/api/sources = %s", rec.Body)
	}
	for _, p := range []string{"/", "/css/style.css", "/js/app.js", "/healthz"} {
		if rec := do(t, h, "GET", p, ""); rec.Code != http.StatusOK {
			t.Errorf("GET %s -> %d", p, rec.Code)
		}
	}
}
