package sources

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func fakeAPI(t *testing.T, status int, file string) (*httptest.Server, *http.Request) {
	t.Helper()
	last := &http.Request{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*last = *r.Clone(context.Background())
		w.WriteHeader(status)
		if file != "" {
			data, err := os.ReadFile("testdata/" + file)
			if err != nil {
				t.Fatal(err)
			}
			w.Write(data)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, last
}

func TestBOESearch(t *testing.T) {
	srv, last := fakeAPI(t, 200, "boe_busqueda.json")
	boe := NewBOE(time.Second)
	boe.BaseURL = srv.URL

	results, err := boe.Search(context.Background(), "trabajo autónomo", 10)
	if err != nil {
		t.Fatal(err)
	}

	if last.URL.Path != "/legislacion-consolidada" {
		t.Errorf("ruta = %q", last.URL.Path)
	}
	if got := last.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
	var q struct {
		Query struct {
			QueryString struct {
				Query string `json:"query"`
			} `json:"query_string"`
		} `json:"query"`
	}
	if err := json.Unmarshal([]byte(last.URL.Query().Get("query")), &q); err != nil {
		t.Fatalf("parámetro query no es JSON: %v", err)
	}
	if want := "titulo:trabajo and titulo:autónomo"; q.Query.QueryString.Query != want {
		t.Errorf("query = %q, quiero %q", q.Query.QueryString.Query, want)
	}
	if last.URL.Query().Get("limit") != "10" {
		t.Errorf("limit = %q", last.URL.Query().Get("limit"))
	}

	if len(results) != 2 {
		t.Fatalf("resultados = %d, quiero 2", len(results))
	}
	r := results[0]
	if r.Source != "boe" || r.ExternalID != "BOE-A-2007-13409" {
		t.Errorf("identificación incorrecta: %+v", r)
	}
	if r.Date != "2007-07-12" {
		t.Errorf("fecha = %q", r.Date)
	}
	if r.URL != "https://www.boe.es/buscar/act.php?id=BOE-A-2007-13409" {
		t.Errorf("url = %q", r.URL)
	}
	if r.Summary != "Ley · Jefatura del Estado · Vigente" {
		t.Errorf("resumen = %q", r.Summary)
	}
	if r.Extra["numero_oficial"] != "20/2007" || r.Extra["actualizada"] != "2026-09-15" {
		t.Errorf("extra = %v", r.Extra)
	}
	if !strings.HasSuffix(results[1].URL, "id=BOE-A-1999-00001") {
		t.Errorf("url de respaldo = %q", results[1].URL)
	}
	if results[1].Extra["vigencia"] != "Vigencia agotada" {
		t.Errorf("vigencia = %q", results[1].Extra["vigencia"])
	}
}

func TestBOENumeroOficial(t *testing.T) {
	boe := NewBOE(time.Second)
	if q := boe.buildQuery("39/2015"); !strings.Contains(q, `numero_oficial:39/2015`) {
		t.Errorf("consulta = %s", q)
	}
}

func TestBOEEmpty(t *testing.T) {
	srv, _ := fakeAPI(t, 200, "boe_vacio.json")
	boe := NewBOE(time.Second)
	boe.BaseURL = srv.URL
	results, err := boe.Search(context.Background(), "nada", 10)
	if err != nil || len(results) != 0 {
		t.Fatalf("resultados = %v, err = %v", results, err)
	}
}

func TestBOEHTTPError(t *testing.T) {
	srv, _ := fakeAPI(t, 500, "")
	boe := NewBOE(time.Second)
	boe.BaseURL = srv.URL
	_, err := boe.Search(context.Background(), "ley", 10)
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v, quiero ErrUpstream", err)
	}
}

func TestDatosGobSearch(t *testing.T) {
	srv, last := fakeAPI(t, 200, "datosgob_busqueda.json")
	dg := NewDatosGob(time.Second)
	dg.BaseURL = srv.URL

	results, err := dg.Search(context.Background(), "empleo público", 5)
	if err != nil {
		t.Fatal(err)
	}
	if want := "/catalog/dataset/title/empleo público.json"; last.URL.Path != want {
		t.Errorf("ruta = %q, quiero %q", last.URL.Path, want)
	}
	if last.URL.Query().Get("_pageSize") != "5" {
		t.Errorf("_pageSize = %q", last.URL.Query().Get("_pageSize"))
	}
	if len(results) != 2 {
		t.Fatalf("resultados = %d, quiero 2 (el que no tiene _about se descarta)", len(results))
	}
	a := results[0]
	if a.ExternalID != "ea0010587-paro-registrado-por-municipio" {
		t.Errorf("id = %q", a.ExternalID)
	}
	if a.Title != "Paro registrado por municipio" {
		t.Errorf("título = %q (debe preferir español)", a.Title)
	}
	if a.Date != "2026-10-15" {
		t.Errorf("fecha = %q", a.Date)
	}
	if a.Extra["publicador"] != "EA0010587" || a.Extra["tematica"] != "empleo, economia" {
		t.Errorf("extra = %v", a.Extra)
	}
	if a.Extra["palabras_clave"] != "paro, empleo" {
		t.Errorf("palabras clave = %q", a.Extra["palabras_clave"])
	}

	b := results[1]
	if b.Title != "Ofertas de empleo público" || b.Summary != "Convocatorias del Ayuntamiento." {
		t.Errorf("título/resumen = %q / %q", b.Title, b.Summary)
	}
	if b.Extra["publicador"] != "Ayuntamiento de Madrid" || b.Date != "2025-06-30" {
		t.Errorf("publicador/fecha = %q / %q", b.Extra["publicador"], b.Date)
	}
	if b.URL != "https://datos.gob.es/catalogo/l01280796-ofertas-de-empleo-publico" {
		t.Errorf("url = %q", b.URL)
	}
}

func TestAnyDate(t *testing.T) {
	cases := map[string]string{
		"2025-06-30T10:00:00Z":          "2025-06-30",
		"Wed, 15 Oct 2026 08:30:00 GMT": "2026-10-15",
		"mié, 15 oct 2026 08:30:00 GMT": "2026-10-15",
		"no es una fecha":               "",
		"":                              "",
	}
	for in, want := range cases {
		if got := anyDate(in); got != want {
			t.Errorf("anyDate(%q) = %q, quiero %q", in, got, want)
		}
	}
}

func TestCleanQuery(t *testing.T) {
	if got := cleanQuery(`  ley "trabajo" (autónomo) or:texto  `); got != "ley trabajo autónomo or texto" {
		t.Errorf("cleanQuery = %q", got)
	}
}

// fakeSource permite probar SearchAll sin red.
type fakeSource struct {
	name    string
	results []Result
	err     error
	delay   time.Duration
}

func (f fakeSource) Name() string  { return f.name }
func (f fakeSource) Label() string { return f.name }
func (f fakeSource) Search(ctx context.Context, q string, limit int) ([]Result, error) {
	select {
	case <-time.After(f.delay):
		return f.results, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestSearchAllOneSourceFails(t *testing.T) {
	ok := fakeSource{name: "a", results: []Result{{Source: "a", Title: "uno"}}}
	broken := fakeSource{name: "b", err: ErrUpstream}
	slow := fakeSource{name: "c", delay: time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	results, errs := SearchAll(ctx, []Source{ok, broken, slow}, "x", 10)

	if len(results) != 1 || results[0].Title != "uno" {
		t.Errorf("resultados = %v", results)
	}
	if len(errs) != 2 || errs[0].Source != "b" || errs[1].Source != "c" {
		t.Fatalf("errores = %v", errs)
	}
	if errs[1].Message != "la fuente tardó demasiado en responder" {
		t.Errorf("mensaje de timeout = %q", errs[1].Message)
	}
}
