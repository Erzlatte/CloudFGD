package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DatosGob consulta el catálogo nacional de datos abiertos (datos.gob.es).
// Documentación: https://datos.gob.es/en/accessible-apidata
type DatosGob struct {
	BaseURL string
	client  *http.Client
}

func NewDatosGob(timeout time.Duration) *DatosGob {
	return &DatosGob{BaseURL: "https://datos.gob.es/apidata", client: newClient(timeout)}
}

func (d *DatosGob) Name() string  { return "datosgob" }
func (d *DatosGob) Label() string { return "datos.gob.es · Catálogo de datos" }

type datosGobResponse struct {
	Result struct {
		Items []map[string]json.RawMessage `json:"items"`
	} `json:"result"`
}

func (d *DatosGob) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	q := cleanQuery(query)
	if q == "" {
		return []Result{}, nil
	}
	params := url.Values{}
	params.Set("_pageSize", strconv.Itoa(limit))
	params.Set("_page", "0")
	endpoint := strings.TrimRight(d.BaseURL, "/") + "/catalog/dataset/title/" + url.PathEscape(q) + ".json?" + params.Encode()

	body, err := getJSON(ctx, d.client, endpoint)
	if err != nil {
		return nil, err
	}
	var res datosGobResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("%w: JSON no válido", ErrUpstream)
	}

	results := make([]Result, 0, len(res.Result.Items))
	for _, item := range res.Result.Items {
		if r, ok := datosGobResult(item); ok {
			results = append(results, r)
		}
	}
	return results, nil
}

func datosGobResult(item map[string]json.RawMessage) (Result, bool) {
	about := text(item["_about"])
	id := lastSegment(about)
	if id == "" {
		return Result{}, false
	}
	link := about
	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
		link = "https://datos.gob.es/es/catalogo/" + url.PathEscape(id)
	}

	extra := map[string]string{}
	if p := publisher(item["publisher"]); p != "" {
		extra["publicador"] = p
	}
	if themes := lastSegments(item["theme"]); len(themes) > 0 {
		extra["tematica"] = strings.Join(themes, ", ")
	}
	if kws := texts(item["keyword"]); len(kws) > 0 {
		if len(kws) > 8 {
			kws = kws[:8]
		}
		extra["palabras_clave"] = strings.Join(kws, ", ")
	}

	date := anyDate(text(item["modified"]))
	if date == "" {
		date = anyDate(text(item["issued"]))
	}
	title := text(item["title"])
	if title == "" {
		title = id
	}
	return Result{
		Source:     "datosgob",
		ExternalID: id,
		Title:      truncate(title, 500),
		Summary:    truncate(text(item["description"]), 600),
		Date:       date,
		URL:        link,
		Extra:      extra,
	}, true
}

// ── Ayudas para leer los campos de datos.gob.es ──
// La API devuelve el mismo campo de varias formas según el conjunto de datos:
//   "texto"
//   {"_value": "texto", "_lang": "es"}
//   [{"_value": "...", "_lang": "en"}, {"_value": "...", "_lang": "es"}]
// Estas funciones aceptan cualquiera de ellas y prefieren el español.

type langValue struct {
	Value string `json:"_value"`
	Lang  string `json:"_lang"`
	About string `json:"_about"`
}

// values devuelve todos los valores de un campo, con su idioma si lo tiene.
func values(raw json.RawMessage) []langValue {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return []langValue{{Value: s}}
	}
	var one langValue
	if json.Unmarshal(raw, &one) == nil && (one.Value != "" || one.About != "") {
		return []langValue{one}
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) == nil {
		var out []langValue
		for _, el := range list {
			out = append(out, values(el)...)
		}
		return out
	}
	return nil
}

func valueOf(v langValue) string {
	if v.Value != "" {
		return v.Value
	}
	return v.About
}

// text devuelve un único texto, en español si existe.
func text(raw json.RawMessage) string {
	vals := values(raw)
	for _, v := range vals {
		if v.Lang == "es" && v.Value != "" {
			return strings.TrimSpace(v.Value)
		}
	}
	for _, v := range vals {
		if s := valueOf(v); s != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// texts devuelve todos los textos en español (o todos, si ninguno indica idioma).
func texts(raw json.RawMessage) []string {
	vals := values(raw)
	var es, all []string
	for _, v := range vals {
		s := strings.TrimSpace(valueOf(v))
		if s == "" {
			continue
		}
		all = append(all, s)
		if v.Lang == "es" {
			es = append(es, s)
		}
	}
	if len(es) > 0 {
		return es
	}
	return all
}

// publisher acepta una URL ("…/Organismo/E05024401") o un objeto con nombre.
func publisher(raw json.RawMessage) string {
	var obj struct {
		Name  json.RawMessage `json:"name"`
		Label json.RawMessage `json:"label"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		if n := text(obj.Name); n != "" {
			return n
		}
		if n := text(obj.Label); n != "" {
			return n
		}
	}
	return lastSegment(text(raw))
}

func lastSegment(s string) string {
	s = strings.TrimRight(strings.TrimSpace(s), "/")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

func lastSegments(raw json.RawMessage) []string {
	var out []string
	for _, v := range texts(raw) {
		if seg := lastSegment(v); seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

// Las fechas de datos.gob.es pueden venir en varios formatos, a veces con meses en español.
var spanishDate = strings.NewReplacer(
	"lun,", "Mon,", "mar,", "Tue,", "mié,", "Wed,", "mie,", "Wed,", "jue,", "Thu,", "vie,", "Fri,", "sáb,", "Sat,", "sab,", "Sat,", "dom,", "Sun,",
	" ene ", " Jan ", " feb ", " Feb ", " mar ", " Mar ", " abr ", " Apr ", " may ", " May ", " jun ", " Jun ",
	" jul ", " Jul ", " ago ", " Aug ", " sep ", " Sep ", " oct ", " Oct ", " nov ", " Nov ", " dic ", " Dec ",
)

var isoPrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

func anyDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if m := isoPrefix.FindString(s); m != "" {
		return m
	}
	en := spanishDate.Replace(s)
	for _, layout := range []string{time.RFC1123, time.RFC1123Z, "Mon, 2 Jan 2006 15:04:05 MST", "02 Jan 2006 15:04:05", "2 Jan 2006"} {
		if t, err := time.Parse(layout, en); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}
