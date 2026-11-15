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

type BOE struct {
	BaseURL string
	client  *http.Client
}

func NewBOE(timeout time.Duration) *BOE {
	return &BOE{BaseURL: "https://www.boe.es/datosabiertos/api", client: newClient(timeout)}
}

func (b *BOE) Name() string  { return "boe" }
func (b *BOE) Label() string { return "BOE · Legislación consolidada" }

type boeResponse struct {
	Status struct {
		Code string `json:"code"`
		Text string `json:"text"`
	} `json:"status"`
	Data json.RawMessage `json:"data"`
}

type boeCode struct {
	Codigo string `json:"codigo"`
	Texto  string `json:"texto"`
}

type boeNorma struct {
	Identificador      string   `json:"identificador"`
	Titulo             string   `json:"titulo"`
	FechaPublicacion   string   `json:"fecha_publicacion"`
	FechaActualizacion string   `json:"fecha_actualizacion"`
	NumeroOficial      string   `json:"numero_oficial"`
	Rango              *boeCode `json:"rango"`
	Departamento       *boeCode `json:"departamento"`
	Estado             *boeCode `json:"estado_consolidacion"`
	VigenciaAgotada    string   `json:"vigencia_agotada"`
	URLHTML            string   `json:"url_html_consolidada"`
	URLELI             string   `json:"url_eli"`
}

var numeroOficial = regexp.MustCompile(`^\d{1,5}/\d{4}$`)

func (b *BOE) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	q := cleanQuery(query)
	if q == "" {
		return []Result{}, nil
	}

	params := url.Values{}
	params.Set("query", buildBOEQuery(q))
	params.Set("limit", strconv.Itoa(limit))
	endpoint := strings.TrimRight(b.BaseURL, "/") + "/legislacion-consolidada?" + params.Encode()

	body, err := getJSON(ctx, b.client, endpoint)
	if err != nil {
		return nil, err
	}

	var res boeResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("%w: JSON no válido", ErrUpstream)
	}
	if res.Status.Code != "" && res.Status.Code != "200" {
		return nil, fmt.Errorf("%w: %s", ErrUpstream, res.Status.Text)
	}

	normas, err := decodeBOEData(res.Data)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(normas))
	for _, n := range normas {
		if n.Identificador == "" {
			continue
		}
		results = append(results, n.toResult())
	}
	return results, nil
}

func buildBOEQuery(q string) string {
	var cond string
	if numeroOficial.MatchString(q) {
		cond = "numero_oficial:" + q
	} else {
		words := strings.Fields(q)
		parts := make([]string, len(words))
		for i, w := range words {
			parts[i] = "titulo:" + w
		}
		cond = strings.Join(parts, " and ")
	}

	body := map[string]any{
		"query": map[string]any{"query_string": map[string]string{"query": cond}},
		"sort":  []map[string]string{{"fecha_publicacion": "desc"}},
	}
	out, _ := json.Marshal(body)
	return string(out)
}

func decodeBOEData(raw json.RawMessage) ([]boeNorma, error) {
	s := strings.TrimSpace(string(raw))
	switch {
	case s == "" || s == "null" || s == "{}" || s == `""`:
		return nil, nil
	case strings.HasPrefix(s, "["):
		var list []boeNorma
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("%w: formato de datos inesperado", ErrUpstream)
		}
		return list, nil
	default:
		var one boeNorma
		if err := json.Unmarshal(raw, &one); err != nil {
			return nil, fmt.Errorf("%w: formato de datos inesperado", ErrUpstream)
		}
		return []boeNorma{one}, nil
	}
}

func (n boeNorma) toResult() Result {
	extra := map[string]string{}
	var summary []string

	if n.Rango != nil && n.Rango.Texto != "" {
		extra["rango"] = n.Rango.Texto
		summary = append(summary, n.Rango.Texto)
	}
	if n.NumeroOficial != "" {
		extra["numero_oficial"] = n.NumeroOficial
	}
	if n.Departamento != nil && n.Departamento.Texto != "" {
		extra["departamento"] = n.Departamento.Texto
		summary = append(summary, n.Departamento.Texto)
	}
	if n.Estado != nil && n.Estado.Texto != "" {
		extra["consolidacion"] = n.Estado.Texto
	}
	switch n.VigenciaAgotada {
	case "N":
		extra["vigencia"] = "Vigente"
	case "S":
		extra["vigencia"] = "Vigencia agotada"
	}
	if v, ok := extra["vigencia"]; ok {
		summary = append(summary, v)
	}
	if d := boeDate(n.FechaActualizacion); d != "" {
		extra["actualizada"] = d
	}

	link := n.URLHTML
	if link == "" {
		link = "https://www.boe.es/buscar/act.php?id=" + url.QueryEscape(n.Identificador)
	}

	return Result{
		Source:     "boe",
		ExternalID: n.Identificador,
		Title:      truncate(n.Titulo, 500),
		Summary:    strings.Join(summary, " · "),
		Date:       boeDate(n.FechaPublicacion),
		URL:        link,
		Extra:      extra,
	}
}

func boeDate(s string) string {
	if len(s) < 8 {
		return ""
	}
	t, err := time.Parse("20060102", s[:8])
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}
