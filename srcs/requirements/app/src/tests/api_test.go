package tests

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"CloudFGD/app/internal/api"
	"CloudFGD/app/internal/sources"
	"CloudFGD/app/internal/store"
)

func TestAPISources(t *testing.T) {
	app := newTestApp(t, stubSource{name: "boe"}, stubSource{name: "datosgob"})

	rec := app.do(t, "GET", api.RouteSources, "")
	var got []api.SourceInfo
	json.NewDecoder(rec.Body).Decode(&got)
	if rec.Code != http.StatusOK || len(got) != 2 || got[1].Name != "datosgob" || got[1].Label == "" {
		t.Errorf("%d %+v", rec.Code, got)
	}
}

func TestAPISearchConErrorParcial(t *testing.T) {
	boe := stubSource{name: "boe", results: []sources.Result{{Source: "boe", ExternalID: "BOE-A-1", Title: "Ley"}}}
	broken := stubSource{name: "datosgob", err: sources.ErrUpstream}
	app := newTestApp(t, boe, broken)

	rec := app.do(t, "GET", "/api/search?q=ley", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("estado = %d: %s", rec.Code, rec.Body)
	}
	var body api.SearchResponse
	json.NewDecoder(rec.Body).Decode(&body)
	if body.Query != "ley" || len(body.Results) != 1 || len(body.Errors) != 1 || body.Errors[0].Source != "datosgob" {
		t.Errorf("respuesta = %+v", body)
	}
}

func TestAPISearchTodasFallanEs502(t *testing.T) {
	app := newTestApp(t, stubSource{name: "boe", err: sources.ErrUpstream})
	if rec := app.do(t, "GET", "/api/search?q=ley", ""); rec.Code != http.StatusBadGateway {
		t.Errorf("estado = %d, quiero 502", rec.Code)
	}
}

func TestAPISearchValidacion(t *testing.T) {
	app := newTestApp(t, stubSource{name: "boe"})
	cases := map[string]int{
		"/api/search":                               http.StatusBadRequest,
		"/api/search?q=%20%20":                      http.StatusBadRequest,
		"/api/search?q=ley&source=nope":             http.StatusBadRequest,
		"/api/search?q=ley&limit=0":                 http.StatusBadRequest,
		"/api/search?q=ley&limit=26":                http.StatusBadRequest,
		"/api/search?q=ley&limit=abc":               http.StatusBadRequest,
		"/api/search?q=" + strings.Repeat("a", 101): http.StatusBadRequest,
		"/api/search?q=ley&source=boe&limit=25":     http.StatusOK,
		"/api/search?q=ley&source=all":              http.StatusOK,
	}
	for target, want := range cases {
		rec := app.do(t, "GET", target, "")
		if rec.Code != want {
			t.Errorf("%s -> %d, quiero %d", target, rec.Code, want)
		}
		if want == http.StatusBadRequest {
			var e api.ErrorResponse
			if json.NewDecoder(rec.Body).Decode(&e); e.Error == "" {
				t.Errorf("%s: falta el mensaje de error", target)
			}
		}
	}
}

const validItem = `{"source":"boe","external_id":"BOE-A-2007-13409","title":"Ley 20/2007",
	"date":"2007-07-12","url":"https://www.boe.es/buscar/act.php?id=BOE-A-2007-13409",
	"tags":[" Autónomos ","trabajo","autónomos"],"note":"Para el TFG"}`

func TestAPICicloDeVidaGuardados(t *testing.T) {
	app := newTestApp(t, stubSource{name: "boe"}, stubSource{name: "datosgob"})

	rec := app.do(t, "POST", api.RouteSaved, validItem)
	if rec.Code != http.StatusCreated {
		t.Fatalf("crear: %d %s", rec.Code, rec.Body)
	}
	var created api.SavedItem
	json.NewDecoder(rec.Body).Decode(&created)
	if created.ID == "" || created.SavedAt.IsZero() {
		t.Errorf("faltan id o fecha de guardado: %+v", created)
	}
	if strings.Join(created.Tags, ",") != "autónomos,trabajo" {
		t.Errorf("etiquetas normalizadas = %v", created.Tags)
	}

	rec = app.do(t, "POST", api.RouteSaved, validItem)
	var dup api.ErrorResponse
	json.NewDecoder(rec.Body).Decode(&dup)
	if rec.Code != http.StatusConflict || dup.ID != created.ID {
		t.Errorf("duplicado: %d %+v", rec.Code, dup)
	}

	other := `{"source":"datosgob","external_id":"x","title":"Paro","url":"https://datos.gob.es/catalogo/x","tags":["empleo"]}`
	if rec := app.do(t, "POST", api.RouteSaved, other); rec.Code != http.StatusCreated {
		t.Fatalf("crear otro: %d %s", rec.Code, rec.Body)
	}

	count := func(target string) int {
		rec := app.do(t, "GET", target, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s -> %d %s", target, rec.Code, rec.Body)
		}
		var list []api.SavedItem
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

	reloaded, err := store.New(app.storePath)
	if err != nil || len(reloaded.List(store.Filter{})) != 2 {
		t.Fatalf("tras recargar: %v", err)
	}

	if rec := app.do(t, "DELETE", "/api/saved/"+created.ID, ""); rec.Code != http.StatusNoContent {
		t.Errorf("borrar: %d", rec.Code)
	}
	if rec := app.do(t, "DELETE", "/api/saved/"+created.ID, ""); rec.Code != http.StatusNotFound {
		t.Errorf("borrar dos veces: %d, quiero 404", rec.Code)
	}
	if got := count("/api/saved"); got != 1 {
		t.Errorf("tras borrar quedan %d", got)
	}
}

func TestAPIGuardarValidacion(t *testing.T) {
	app := newTestApp(t, stubSource{name: "boe"})
	bad := map[string]string{
		"no es JSON":           `no es json`,
		"fuente desconocida":   `{"source":"otra","external_id":"1","title":"t","url":"https://a.es"}`,
		"sin identificador":    `{"source":"boe","external_id":"","title":"t","url":"https://a.es"}`,
		"sin título":           `{"source":"boe","external_id":"1","title":"","url":"https://a.es"}`,
		"enlace javascript":    `{"source":"boe","external_id":"1","title":"t","url":"javascript:alert(1)"}`,
		"fecha mal formada":    `{"source":"boe","external_id":"1","title":"t","url":"https://a.es","date":"12/07/2007"}`,
		"etiqueta larga":       `{"source":"boe","external_id":"1","title":"t","url":"https://a.es","tags":["` + strings.Repeat("x", 31) + `"]}`,
		"demasiadas etiquetas": `{"source":"boe","external_id":"1","title":"t","url":"https://a.es","tags":["a","b","c","d","e","f","g","h","i","j","k"]}`,
		"nota larga":           `{"source":"boe","external_id":"1","title":"t","url":"https://a.es","note":"` + strings.Repeat("n", 1001) + `"}`,
	}
	for name, body := range bad {
		if rec := app.do(t, "POST", api.RouteSaved, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s -> %d, quiero 400", name, rec.Code)
		}
	}

	filters := []string{"/api/saved?from=ayer", "/api/saved?to=2020-1-1", "/api/saved?source=nope"}
	for _, target := range filters {
		if rec := app.do(t, "GET", target, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s -> %d, quiero 400", target, rec.Code)
		}
	}
}

func TestAPIFrontendYSalud(t *testing.T) {
	app := newTestApp(t, stubSource{name: "boe"})
	pages := map[string]string{
		"/":              "text/html",
		"/css/style.css": "text/css",
		"/js/app.js":     "javascript",
		"/js/api.js":     "javascript",
		"/healthz":       "text/plain",
	}
	for path, mime := range pages {
		rec := app.do(t, "GET", path, "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), mime) {
			t.Errorf("GET %s -> %d %q", path, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	if rec := app.do(t, "GET", "/no-existe", ""); rec.Code != http.StatusNotFound {
		t.Errorf("ruta desconocida -> %d", rec.Code)
	}
	if rec := app.do(t, "PUT", api.RouteSaved, ""); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("método no permitido -> %d", rec.Code)
	}
}
