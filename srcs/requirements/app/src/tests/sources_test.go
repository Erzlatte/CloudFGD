package tests

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"CloudFGD/app/internal/sources"
)

func boeQueryCondition(t *testing.T, raw string) string {
	t.Helper()
	var q struct {
		Query struct {
			QueryString struct {
				Query string `json:"query"`
			} `json:"query_string"`
		} `json:"query"`
	}
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		t.Fatalf("parámetro query no es JSON: %v", err)
	}
	return q.Query.QueryString.Query
}

func TestBOESearch(t *testing.T) {
	srv, last := fakeAPI(t, 200, "boe_busqueda.json")
	boe := sources.NewBOE(time.Second)
	boe.BaseURL = srv.URL

	results, err := boe.Search(context.Background(), "trabajo autónomo", 10)
	if err != nil {
		t.Fatal(err)
	}

	if last.req.URL.Path != "/legislacion-consolidada" {
		t.Errorf("ruta = %q", last.req.URL.Path)
	}
	if got := last.req.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
	if got := boeQueryCondition(t, last.req.URL.Query().Get("query")); got != "titulo:trabajo and titulo:autónomo" {
		t.Errorf("condición = %q", got)
	}
	if last.req.URL.Query().Get("limit") != "10" {
		t.Errorf("limit = %q", last.req.URL.Query().Get("limit"))
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

func TestBOEBuscaPorNumeroOficial(t *testing.T) {
	srv, last := fakeAPI(t, 200, "boe_vacio.json")
	boe := sources.NewBOE(time.Second)
	boe.BaseURL = srv.URL

	if _, err := boe.Search(context.Background(), "39/2015", 10); err != nil {
		t.Fatal(err)
	}
	if got := boeQueryCondition(t, last.req.URL.Query().Get("query")); got != "numero_oficial:39/2015" {
		t.Errorf("condición = %q", got)
	}
}

func TestBOELimpiaCaracteresPeligrosos(t *testing.T) {
	srv, last := fakeAPI(t, 200, "boe_vacio.json")
	boe := sources.NewBOE(time.Second)
	boe.BaseURL = srv.URL

	if _, err := boe.Search(context.Background(), `  ley "trabajo" (autónomo) or:texto  `, 10); err != nil {
		t.Fatal(err)
	}
	want := "titulo:ley and titulo:trabajo and titulo:autónomo and titulo:or and titulo:texto"
	if got := boeQueryCondition(t, last.req.URL.Query().Get("query")); got != want {
		t.Errorf("condición = %q, quiero %q", got, want)
	}
}

func TestBOESinResultados(t *testing.T) {
	srv, _ := fakeAPI(t, 200, "boe_vacio.json")
	boe := sources.NewBOE(time.Second)
	boe.BaseURL = srv.URL

	results, err := boe.Search(context.Background(), "nada", 10)
	if err != nil || len(results) != 0 {
		t.Fatalf("resultados = %v, err = %v", results, err)
	}
}

func TestBOEErrorHTTP(t *testing.T) {
	srv, _ := fakeAPI(t, 500, "")
	boe := sources.NewBOE(time.Second)
	boe.BaseURL = srv.URL

	if _, err := boe.Search(context.Background(), "ley", 10); !errors.Is(err, sources.ErrUpstream) {
		t.Fatalf("err = %v, quiero ErrUpstream", err)
	}
}

func TestDatosGobSearch(t *testing.T) {
	srv, last := fakeAPI(t, 200, "datosgob_busqueda.json")
	dg := sources.NewDatosGob(time.Second)
	dg.BaseURL = srv.URL

	results, err := dg.Search(context.Background(), "empleo público", 5)
	if err != nil {
		t.Fatal(err)
	}
	if want := "/catalog/dataset/title/empleo público.json"; last.req.URL.Path != want {
		t.Errorf("ruta = %q, quiero %q", last.req.URL.Path, want)
	}
	if last.req.URL.Query().Get("_pageSize") != "5" {
		t.Errorf("_pageSize = %q", last.req.URL.Query().Get("_pageSize"))
	}
	if len(results) != 2 {
		t.Fatalf("resultados = %d, quiero 2", len(results))
	}

	a := results[0]
	if a.ExternalID != "ea0010587-paro-registrado-por-municipio" {
		t.Errorf("id = %q", a.ExternalID)
	}
	if a.Title != "Paro registrado por municipio" {
		t.Errorf("título = %q", a.Title)
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

func TestDatosGobFechas(t *testing.T) {
	srv, _ := fakeAPI(t, 200, "datosgob_fechas.json")
	dg := sources.NewDatosGob(time.Second)
	dg.BaseURL = srv.URL

	results, err := dg.Search(context.Background(), "fechas", 10)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"fecha-en-espanol": "2026-10-15",
		"fecha-no-valida":  "",
		"solo-issued":      "2019-03-01",
	}
	if len(results) != len(want) {
		t.Fatalf("resultados = %d", len(results))
	}
	for _, r := range results {
		if r.Date != want[r.ExternalID] {
			t.Errorf("%s: fecha = %q, quiero %q", r.ExternalID, r.Date, want[r.ExternalID])
		}
	}
}

func TestSearchAllUnaFuenteFalla(t *testing.T) {
	ok := stubSource{name: "a", results: []sources.Result{{Source: "a", Title: "uno"}}}
	broken := stubSource{name: "b", err: sources.ErrUpstream}
	slow := stubSource{name: "c", delay: time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	results, errs := sources.SearchAll(ctx, []sources.Source{ok, broken, slow}, "x", 10)

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
