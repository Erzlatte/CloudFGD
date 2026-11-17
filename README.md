# CloudFGD

CloudFGD es una aplicación web escrita en **Go** y desplegada con **Docker**. Busca a la vez en dos
fuentes oficiales de información pública (el **BOE** y **datos.gob.es**), muestra los resultados
juntos y permite **guardar** lo que interesa con etiquetas y una nota, para consultarlo y filtrarlo
después. Incluye una pestaña de **ayuda** con preguntas frecuentes.

Este documento explica cómo está hecho el código: qué hace cada fichero, cómo se conectan las piezas
y cómo se construye, se prueba y se arranca.

---

## Índice

1. [Arquitectura](#1-arquitectura)
2. [Estructura del proyecto](#2-estructura-del-proyecto)
3. [Recorrido de una petición](#3-recorrido-de-una-petición)
4. [Servicio `app`: backend en Go](#4-servicio-app-backend-en-go)
5. [Conectores a las APIs públicas](#5-conectores-a-las-apis-públicas)
6. [Conexión frontend ↔ backend](#6-conexión-frontend--backend)
7. [Frontend: HTML, CSS y JavaScript](#7-frontend-html-css-y-javascript)
8. [Servicio `proxy`](#8-servicio-proxy)
9. [Tests](#9-tests)
10. [Docker, Compose, `.env` y Makefile](#10-docker-compose-env-y-makefile)
11. [API REST](#11-api-rest)
12. [Problemas frecuentes](#12-problemas-frecuentes)
13. [Referencias de las herramientas](#13-referencias-de-las-herramientas)

---

## 1. Arquitectura

```
                 Host                               Red Docker "cloudfgd"                        Internet
 ┌─────────────────────────┐      ┌──────────────────────────────────────────────┐
 │  Navegador              │ 443  │  ┌────────────┐  HTTP  ┌───────────────────┐ │  HTTPS  ┌──────────────┐
 │  https://localhost  ────┼──────┼─>│   proxy    │───────>│       app         │─┼────────>│ API del BOE  │
 │                         │ TLS  │  │ (Go, :443) │ :8080  │    (Go, :8080)    │─┼────────>│ datos.gob.es │
 └─────────────────────────┘      │  └─────┬──────┘        └─────────┬─────────┘ │         └──────────────┘
                                  └────────┼─────────────────────────┼───────────┘
                                           ▼ /certs                  ▼ /data
                                     ~/data/certs           ~/data/app/saved.json
```

| Servicio | Puerto | Expuesto | Responsabilidad |
|----------|--------|----------|-----------------|
| `proxy` | 443 | **Sí** (único) | Cifra con TLS, genera su certificado, reenvía todo a la `app` y registra los accesos |
| `app` | 8080 | No | Sirve la web, expone la API REST, consulta las fuentes públicas y guarda los elementos |

El navegador **solo habla con el proxy**, y solo la `app` llama a las APIs externas. Ambos servicios
usan **únicamente la librería estándar de Go**: no hay dependencias externas.

### Capas dentro de la `app`

```
                        ┌──────────────────────────────┐
     navegador ───────> │  web/  (HTML · CSS · JS)     │
                        │    js/api.js  ───────────────┼──┐
                        └──────────────────────────────┘  │ fetch (JSON)
                                                          ▼
 ┌──────────┐   crea   ┌──────────────────────────────────────────┐
 │ main.go  │ ───────> │ internal/api                              │
 │          │          │   routes.go     qué ruta va a qué función │
 │          │          │   contract.go   forma de cada JSON        │
 │          │          │   handle_*.go   qué hace cada ruta        │
 │          │          │   validate.go   qué datos se aceptan      │
 └────┬─────┘          └───────────┬─────────────────┬────────────┘
      │ lee                        │ usa             │ usa
      ▼                            ▼                 ▼
 ┌──────────────┐        ┌──────────────────┐  ┌──────────────────────┐
 │ internal/    │        │ internal/store   │  │ internal/sources     │
 │   config     │        │ saved.json       │  │ boe.go · datosgob.go │──> APIs públicas
 └──────────────┘        └──────────────────┘  └──────────────────────┘
```

Reglas de esta división:

- `api` no sabe cómo es el BOE: solo conoce la interfaz `sources.Source`.
- `store` no sabe nada de HTTP: recibe y devuelve estructuras Go.
- `sources` no sabe nada del almacén ni de la web: solo traduce APIs externas.
- El frontend solo habla con el servidor a través de `js/api.js`.
- Los tests viven aparte, en `tests/`, y solo usan lo que cada paquete exporta.

### Convenciones

- El código **no lleva comentarios**; la explicación está en este documento. La única línea que
  empieza por `//` es `//go:embed`, una instrucción para el compilador.
- En Go, lo que empieza por mayúscula (`New`, `SavedItem`) es público; lo que empieza por minúscula
  (`persist`, `cleanQuery`) es interno de su paquete.
- Los paquetes dentro de `internal/` solo se pueden importar desde el propio módulo.

---

## 2. Estructura del proyecto

```
CloudFGD/
├── Makefile
├── README.md
└── srcs/
    ├── .env
    ├── docker-compose.yml
    └── requirements/
        ├── proxy/
        │   ├── Dockerfile
        │   └── src/
        │       ├── go.mod
        │       ├── main.go            proxy inverso y servidor TLS
        │       ├── cert.go            certificado autofirmado
        │       └── healthcheck.go     comprobación para Docker
        └── app/
            ├── Dockerfile             etapas build, test y final
            └── src/
                ├── go.mod             módulo CloudFGD/app, Go 1.22+
                ├── main.go            arranque y montaje de piezas
                ├── internal/
                │   ├── api/
                │   │   ├── routes.go          tabla de rutas y servidor
                │   │   ├── contract.go        rutas, límites y tipos JSON
                │   │   ├── respond.go         escritura de respuestas JSON
                │   │   ├── validate.go        validación de lo que llega
                │   │   ├── handle_sources.go  GET /api/sources
                │   │   ├── handle_search.go   GET /api/search
                │   │   ├── handle_saved.go    GET, POST y DELETE /api/saved
                │   │   └── handle_health.go   GET /healthz
                │   ├── config/config.go       variables de entorno
                │   ├── health/health.go       healthcheck
                │   ├── sources/
                │   │   ├── source.go          interfaz común y utilidades
                │   │   ├── search.go          búsqueda en paralelo
                │   │   ├── boe.go             conector del BOE
                │   │   └── datosgob.go        conector de datos.gob.es
                │   └── store/store.go         guardados en saved.json
                ├── web/
                │   ├── web.go                 incrusta el frontend en el binario
                │   ├── html/index.html
                │   ├── css/style.css
                │   └── js/
                │       ├── app.js             arranque
                │       ├── api.js             conexión con el servidor
                │       ├── tabs.js            pestañas
                │       ├── search.js          buscador y resultados
                │       ├── saved.js           lista de guardados
                │       ├── state.js           datos compartidos
                │       └── dom.js             utilidades de HTML
                └── tests/
                    ├── helpers_test.go
                    ├── api_test.go
                    ├── sources_test.go
                    ├── store_test.go
                    ├── health_config_test.go
                    └── testdata/              respuestas de ejemplo de las APIs
```

---

## 3. Recorrido de una petición

### Buscar

1. El usuario escribe `trabajo autónomo` (o pulsa un ejemplo) y pulsa *Buscar*.
2. `search.js` llama a `searchSources({ q, source })` de `api.js`.
3. `api.js` hace `GET /api/search?q=trabajo+autónomo&source=all`.
4. El proxy recibe la petición cifrada y la reenvía por HTTP a la `app`.
5. `routes.go` la dirige a `handleSearch`, que valida los parámetros.
6. `sources.SearchAll` lanza una goroutine por fuente: el BOE y datos.gob.es se consultan a la vez.
7. Cada conector traduce la respuesta de su API al formato común `Result`.
8. `handleSearch` responde un `SearchResponse`: resultados y, aparte, las fuentes que fallaron.
9. `search.js` pinta una tarjeta por resultado clonando un `<template>` del HTML.

### Guardar

1. En una tarjeta: *Guardar* → etiquetas y nota → *Confirmar*.
2. `saveResult(result, { tags, note })` envía `POST /api/saved` con un `SaveRequest`.
3. `handleCreateSaved` decodifica el JSON y `validateSave` lo comprueba.
4. `store.Add` lo añade a la lista y reescribe `saved.json`.
5. Respuesta `201` (o `409` si ya existía); la tarjeta pasa a *Guardado ✓*.

### Consultar y borrar

1. En *Guardados*, cada cambio de filtro llama a `listSaved()` → `GET /api/saved?source=&tag=&from=&to=`.
2. `store.List` filtra en memoria y devuelve la lista, del más reciente al más antiguo.
3. *Borrar* llama a `deleteSaved(id)` → `DELETE /api/saved/{id}` → `store.Delete`.

---

## 4. Servicio `app`: backend en Go

Código en `srcs/requirements/app/src/`.

### 4.1 `main.go`

No tiene lógica propia: lee la configuración, crea cada pieza y las conecta.

| Paso | Qué hace |
|------|----------|
| `flag.Bool("healthcheck", …)` | Define el parámetro `-healthcheck` que usa Docker |
| `config.Load()` | Lee las variables de entorno; si `SEARCH_TIMEOUT` no es válido, termina con error |
| Modo healthcheck | Si se arrancó con `-healthcheck`, no levanta el servidor: llama a `health.Check` contra su propio `/healthz` y sale con 0 o 1 |
| `store.New(…/saved.json)` | Abre el almacén de guardados |
| `api.New(api.Deps{…})` | Construye el manejador HTTP con el almacén, los conectores, el tiempo máximo y el frontend |
| `http.Server{ReadHeaderTimeout: 5s}` | Servidor HTTP; corta a los clientes que envían las cabeceras demasiado despacio |
| `ListenAndServe()` | Arranca. Si falla, el programa termina y Docker lo reinicia |

`buildSources(cfg)` crea los conectores del BOE y de datos.gob.es y, si `BOE_BASE_URL` o
`DATOSGOB_BASE_URL` tienen valor, los apunta a esa URL (para pruebas con una API simulada). El orden
de la lista es el orden en que aparecen las fuentes en la web. **Añadir una fuente nueva es añadirla
aquí.**

### 4.2 `internal/config/config.go`

| Campo de `Config` | Variable | Por defecto |
|-------------------|----------|-------------|
| `Port` | `APP_PORT` | `8080` |
| `DataDir` | `DATA_PATH` | `/data` |
| `SearchTimeout` | `SEARCH_TIMEOUT` | `10s` (se interpreta con `time.ParseDuration`: `10s`, `1m`, `500ms`…) |
| `BOEBaseURL` | `BOE_BASE_URL` | vacío |
| `DatosGobBaseURL` | `DATOSGOB_BASE_URL` | vacío |

`Load()` devuelve error si el tiempo máximo no se puede interpretar o no es positivo. `get(key, fallback)`
devuelve la variable o el valor por defecto si está vacía.

### 4.3 `internal/api`

#### `routes.go`

- **`Deps`**: lo que la API necesita (`Store`, `Sources`, `SearchTimeout`, `Static`). Se lo pasa
  `main.go`; los tests le pasan versiones de prueba.
- **`Server`**: copia privada de las dependencias. Todos los handlers son métodos de `Server`, así
  no hay variables globales.
- **`New(d Deps) http.Handler`**: crea el `Server` y devuelve su enrutador.
- **`routes()`**: la tabla de rutas.

| Patrón | Handler | Qué hace |
|--------|---------|----------|
| `GET /{$}` | `handleIndex` | Página principal (`{$}` = solo `/` exacto; lo demás da 404) |
| `GET /css/`, `GET /js/` | `http.FileServer` | Ficheros estáticos incrustados |
| `GET /api/sources` | `handleSources` | Fuentes disponibles |
| `GET /api/search` | `handleSearch` | Búsqueda |
| `GET /api/saved` | `handleListSaved` | Lista de guardados |
| `POST /api/saved` | `handleCreateSaved` | Guardar |
| `DELETE /api/saved/{id}` | `handleDeleteSaved` | Borrar (`{id}` se lee con `r.PathValue("id")`) |
| `GET /healthz` | `handleHealth` | Salud |

Como cada patrón incluye el método, un `PUT /api/saved` recibe `405` automáticamente.
`sourceByName(name)` busca un conector por nombre y lo usan la búsqueda, los filtros y la validación.

#### `contract.go`

Define todo lo que el frontend necesita saber del servidor: las rutas (`RouteSources`, `RouteSearch`,
`RouteSaved`, `RouteSavedItem`, `RouteHealth`), los límites (`MaxQueryLen`, `MaxLimit`, `MaxTags`…) y
los tipos JSON (`SourceInfo`, `SearchResponse`, `SaveRequest`, `SavedItem`, `ErrorResponse`). Se
detalla en la [sección 6](#6-conexión-frontend--backend).

#### `respond.go`

- `writeJSON(w, status, v)`: pone `Content-Type: application/json; charset=utf-8`, el código HTTP y
  codifica `v`.
- `writeError(w, status, msg)`: responde `{"error": msg}`.

Usar siempre estas dos funciones garantiza que todas las respuestas tienen el mismo formato.

#### `handle_sources.go`

`handleSources` devuelve el `Name()` y el `Label()` de cada conector como `[]SourceInfo`.

#### `handle_search.go`

| Paso | Comprobación | Si falla |
|------|--------------|----------|
| 1 | `q` sin espacios sobrantes, no vacío | `400` |
| 2 | `q` de como mucho 100 caracteres (se cuentan letras, no bytes, para que "ñ" cuente como una) | `400` |
| 3 | `parseLimit`: vacío → 10; si no, número entre 1 y 25 | `400` |
| 4 | `source`: vacío o `all` → todas; otro valor → esa fuente | `400` si no existe |
| 5 | Crea un `context` con `SEARCH_TIMEOUT` (también se cancela si el navegador abandona) | — |
| 6 | `sources.SearchAll` y escribe en el log las fuentes que fallaron | — |
| 7 | Sin resultados **y** todas las fuentes fallaron → `502`; si no → `200` | — |

#### `handle_saved.go`

- **`handleListSaved`**: construye un `store.Filter` con `source`, `tag` (normalizada igual que al
  guardar, así `?tag=EMPLEO` encuentra `empleo`), `from` y `to`; valida la fuente y que las fechas sean
  `AAAA-MM-DD`; devuelve `store.List`.
- **`handleCreateSaved`**: limita el cuerpo a 16 KB, decodifica un `SaveRequest`, lo valida y llama a
  `store.Add`. Duplicado → `409` con el `id` existente; error de disco → `500`; bien → `201`.
- **`handleDeleteSaved`**: `store.Delete`; no existe → `404`; bien → `204`.

#### `handle_health.go`

`handleHealth` responde `200 ok` si `store.Writable()` funciona y `503` si no. No consulta las APIs
externas a propósito: si el BOE se cae, la aplicación sigue sana.

#### `validate.go`

Todo lo que llega del navegador se comprueba, aunque la web ya lo haga: cualquiera puede enviar una
petición a mano.

| Campo | Regla |
|-------|-------|
| `source` | Fuente registrada |
| `external_id` | Obligatorio, máx. 200 |
| `title` | Obligatorio, máx. 500 |
| `summary`, `note` | Máx. 1000 |
| `date` | Vacía o `AAAA-MM-DD` |
| `url` | `http`/`https` con dominio (`isHTTPURL`). Impide guardar un `javascript:` |
| `tags` | `normalizeTags`: minúsculas, un espacio entre palabras, sin vacías ni repetidas, máx. 10 de 30 caracteres |
| `extra` | `validateExtra`: máx. 10 campos, claves de 50 y valores de 300 caracteres |

### 4.4 `internal/store/store.go`

Guarda la lista en memoria y la copia completa en `saved.json` cada vez que cambia.

- **`SavedItem`**: los campos de un resultado (`source`, `external_id`, `title`, `summary`, `date`,
  `url`, `extra`) más `id` (16 caracteres hexadecimales aleatorios), `tags`, `note` y `saved_at`.
- **`Filter`**: fuente, etiqueta y rango de fechas. Un campo vacío no filtra.
- **`Store`**: un `sync.Mutex` (el servidor atiende cada petición en una goroutine distinta y el mutex
  hace que solo una a la vez toque la lista), la ruta del fichero y la lista, del más antiguo al más
  reciente.
- **`ErrDuplicate`** y **`ErrNotFound`**: la API los reconoce con `errors.Is` para elegir el código HTTP.

| Función | Qué hace |
|---------|----------|
| `New(path)` | Lee el fichero; si no existe, empieza vacío. Si no es JSON válido, devuelve error en lugar de sobrescribirlo |
| `Add(it)` | Si ya hay uno con la misma fuente e identificador, devuelve ese y `ErrDuplicate`. Si no, le asigna `id` y `saved_at`, construye una lista nueva, la escribe y solo entonces la sustituye en memoria |
| `Delete(id)` | Igual, pero quitando el elemento. Si no existe, `ErrNotFound` |
| `List(f)` | Recorre la lista al revés y devuelve los que cumplen el filtro. Nunca devuelve `nil`, para que el JSON sea `[]` |
| `Writable()` | Crea y borra un fichero temporal en la carpeta de datos |
| `persist(items)` | Escritura **atómica**: escribe `saved.json.tmp` y lo renombra a `saved.json`. Si el contenedor se cae a mitad, el fichero nunca queda a medias |

Construir una lista nueva en `Add` y `Delete` asegura que, si la escritura falla, memoria y disco no
quedan distintos. Limitación: solo un proceso puede escribir el fichero; con varias réplicas de la
app habría que sustituir `store` por una base de datos.

### 4.5 `internal/health/health.go`

`Check(url, insecure)` hace un `GET` con 3 segundos de límite y devuelve `Healthy` (0) si la
respuesta es `200` o `Unhealthy` (1) en cualquier otro caso, que son los códigos que interpreta la
instrucción `HEALTHCHECK` de Docker. `insecure` acepta certificados autofirmados.

### 4.6 `web/web.go`

```go
//go:embed html css js
var files embed.FS
```

La directiva `//go:embed` mete las carpetas del frontend dentro del ejecutable al compilar. `Files()`
las expone como un sistema de ficheros de solo lectura. Por eso la imagen de Docker solo necesita el
binario.

---

## 5. Conectores a las APIs públicas

Código en `internal/sources/`. Un conector habla con una API concreta y traduce su respuesta a un
formato común.

### 5.1 `source.go`: el contrato común

```go
type Source interface {
	Name() string
	Label() string
	Search(ctx context.Context, query string, limit int) ([]Result, error)
}
```

`Name()` es el identificador que se usa en `?source=` (`boe`), `Label()` el texto que ve el usuario y
`Search` devuelve hasta `limit` resultados respetando `ctx`. Cualquier tipo con estos tres métodos es
un `Source`, por eso la API trabaja igual con el BOE, con datos.gob.es o con una fuente falsa en los tests.

`Result` es el formato común: `source`, `external_id`, `title`, `summary`, `date` (`AAAA-MM-DD` o
vacía, nunca inventada), `url` (enlace oficial) y `extra` (datos propios de cada fuente).

| Utilidad | Qué hace |
|----------|----------|
| `ErrUpstream` | Error "la API externa falló". Se envuelve con detalle y se reconoce con `errors.Is` |
| `getJSON` | `GET` ligado a `ctx` con `Accept: application/json` y `User-Agent` propio; lee como mucho 5 MB; si falla la red o el código no es `200`, devuelve `ErrUpstream` |
| `newClient` | Cliente HTTP con tiempo máximo |
| `truncate` | Corta un texto sin partir letras con tilde |
| `cleanQuery` | Quita todo lo que no sea letra, número, espacio, `/`, `-` o `.`, para que comillas o `:` no cambien la búsqueda |

### 5.2 `search.go`: varias fuentes a la vez

`SearchAll` lanza una goroutine por fuente, cada una escribe en su propio hueco de un array (sin
mutex, porque ninguna comparte hueco) y un `sync.WaitGroup` espera a todas. Después junta los
resultados en el orden de las fuentes y convierte cada error en un `SourceError`
(`{"source", "message"}`). Todas comparten el `context` con el tiempo máximo: la que tarda demasiado
se cancela, pero las que ya respondieron se devuelven.

| Error | Mensaje para el usuario |
|-------|-------------------------|
| Tiempo agotado | `la fuente tardó demasiado en responder` |
| `ErrUpstream` | `la fuente externa no respondió correctamente: <detalle>` |
| Otro | `error inesperado al consultar la fuente` |

### 5.3 `boe.go`: BOE, legislación consolidada

Petición:

```
GET https://www.boe.es/datosabiertos/api/legislacion-consolidada?query=<JSON>&limit=10
```

`buildBOEQuery` construye el JSON del parámetro `query`:

| Texto escrito | Condición enviada |
|---------------|-------------------|
| `trabajo autónomo` | `titulo:trabajo and titulo:autónomo` |
| `39/2015` | `numero_oficial:39/2015` |

y pide orden por fecha de publicación descendente. `decodeBOEData` acepta las tres formas en que el
BOE devuelve `data`: lista, objeto vacío (sin resultados) u objeto único. `toResult` traduce cada norma:

| Campo del BOE | En `Result` |
|---------------|-------------|
| `identificador` | `external_id` |
| `titulo` | `title` |
| `fecha_publicacion` (`20070712`) | `date` (`2007-07-12`), con `boeDate` |
| `url_html_consolidada` | `url` (si falta, se construye con el identificador) |
| rango · departamento · vigencia | `summary`: `Ley · Jefatura del Estado · Vigente` |
| rango, número oficial, departamento, consolidación, vigencia, actualización | `extra` |

### 5.4 `datosgob.go`: datos.gob.es

Petición:

```
GET https://datos.gob.es/apidata/catalog/dataset/title/{texto}.json?_pageSize=10&_page=0
```

El reto de esta API es que **un mismo campo llega de tres formas** según el conjunto de datos:

```json
"title": "Ofertas de empleo público"
"title": { "_value": "Ofertas de empleo público", "_lang": "es" }
"title": [ { "_value": "Public job offers", "_lang": "en" }, { "_value": "Ofertas de empleo público", "_lang": "es" } ]
```

Los campos se leen como `json.RawMessage` y unas pocas funciones aceptan cualquiera de las tres:
`values` (normaliza a una lista), `text` (un texto, preferiblemente en español), `texts` (todos los
textos en español), `publisher` (objeto con nombre o URL) y `lastSegment` (último tramo de una URL).

| Campo | En `Result` |
|-------|-------------|
| `_about` | `url`; su último tramo es `external_id`. Sin `_about` el elemento se descarta |
| `title`, `description` | `title`, `summary` |
| `modified` (o `issued`) | `date`, con `anyDate` |
| `publisher`, `theme`, `keyword` | `extra`: publicador, temática y palabras clave |

`anyDate` entiende fechas ISO (`2025-06-30T10:00:00Z`), de correo (`Wed, 15 Oct 2026 08:30:00 GMT`)
y con días y meses en español (`mié, 15 oct 2026 …`). Si no reconoce el formato, deja la fecha vacía.

### 5.5 Añadir una fuente

1. Crear `internal/sources/<fuente>.go` con un tipo que tenga `Name`, `Label` y `Search`, usando
   `getJSON`, `cleanQuery` y devolviendo `[]Result`.
2. Añadirlo en `buildSources` de `main.go`. Aparece solo en `/api/sources`, en el selector y en las búsquedas.
3. Añadir un test con una respuesta real en `tests/testdata/`.
4. Opcional: su color en `style.css` (`.badge[data-source="…"]`) y los nombres de sus campos `extra`
   en `EXTRA_LABELS` (`state.js`).

---

## 6. Conexión frontend ↔ backend

La conexión está concentrada en **un fichero a cada lado**:

| Lado | Fichero | Papel |
|------|---------|-------|
| Navegador | `web/js/api.js` | Único fichero que llama al servidor. El resto del frontend usa sus funciones |
| Servidor | `internal/api/contract.go` | Rutas, límites y forma exacta de cada JSON |

```
 search.js ─┐                                         routes.go
 saved.js  ─┼─> api.js ── fetch ── proxy ── app ──>   ├─ GET    /api/sources    → handleSources
 app.js    ─┘   getSources()                          ├─ GET    /api/search     → handleSearch
                searchSources()                       ├─ GET    /api/saved      → handleListSaved
                listSaved()                           ├─ POST   /api/saved      → handleCreateSaved
                saveResult()                          └─ DELETE /api/saved/{id} → handleDeleteSaved
                deleteSaved()
                     ▲
                     └── { ok, status, data }  <── SourceInfo · SearchResponse · SavedItem · ErrorResponse
```

| Función en `api.js` | Petición | Handler | Envía | Recibe |
|---------------------|----------|---------|-------|--------|
| `getSources()` | `GET /api/sources` | `handleSources` | — | `[]SourceInfo` |
| `searchSources({ q, source, limit })` | `GET /api/search` | `handleSearch` | parámetros de URL | `SearchResponse` |
| `listSaved({ source, tag, from, to })` | `GET /api/saved` | `handleListSaved` | parámetros de URL | `[]SavedItem` |
| `saveResult(result, { tags, note })` | `POST /api/saved` | `handleCreateSaved` | `SaveRequest` | `SavedItem` o `ErrorResponse` |
| `deleteSaved(id)` | `DELETE /api/saved/{id}` | `handleDeleteSaved` | `id` en la ruta | nada (`204`) o `ErrorResponse` |

### Lado del navegador (`api.js`)

- `ROUTES` contiene las mismas rutas que las constantes `Route*` de `contract.go`.
- `request(method, path, body)` es la **única llamada a `fetch`**. Si hay cuerpo lo envía como JSON;
  lee la respuesta como JSON (o `null` si es `204`) y devuelve **siempre** `{ ok, status, data }`, sin
  lanzar excepciones. Si no hay conexión: `status: 0` y `data.error` con el motivo.
- `withQuery(path, params)` construye los parámetros con `URLSearchParams` y omite los vacíos.
- `saveResult` construye el cuerpo campo a campo con la forma de `SaveRequest`.
- Las rutas son relativas: la web y la API las sirve el mismo servidor, así que no hace falta
  configurar URLs ni CORS.

### Lado del servidor (`contract.go`)

```json
SourceInfo      { "name": "boe", "label": "BOE · Legislación consolidada" }
SearchResponse  { "query": "…", "results": [Result…], "errors": [{ "source": "…", "message": "…" }] }
SaveRequest     { "source", "external_id", "title", "summary", "date", "url", "extra", "tags", "note" }
SavedItem       SaveRequest + { "id", "saved_at" }
ErrorResponse   { "error": "…" }  ·  en el 409 también { "id": "…" }
```

| Constante | Valor |
|-----------|-------|
| `MaxQueryLen` | 100 |
| `DefaultLimit` / `MaxLimit` | 10 / 25 |
| `MaxExternalID` / `MaxTitleLen` | 200 / 500 |
| `MaxSummaryLen` / `MaxNoteLen` | 1000 / 1000 |
| `MaxTags` / `MaxTagLen` | 10 / 30 |
| `MaxExtraFields` / `MaxExtraKeyLen` / `MaxExtraValLen` | 10 / 50 / 300 |
| `MaxBodyBytes` | 16 KB |

### Cómo trata la web cada código

| Código | Qué hace la interfaz |
|--------|----------------------|
| `200` | Pinta los datos |
| `201` / `409` | Marca *Guardado ✓* (409 = "Ya estaba guardado") y actualiza el contador |
| `204` | Recarga la lista |
| `400` / `404` | Muestra `data.error` |
| `502` | Avisos por fuente y "No se pudo consultar ninguna fuente" |
| `0` | "No se pudo conectar con el servidor" |

---

## 7. Frontend: HTML, CSS y JavaScript

Código en `web/`, sin frameworks.

### 7.1 Pantallas

- **Buscar:** texto, selector de fuente, ejemplos de un clic («Prueba con:»), guía de tres pasos
  antes de la primera búsqueda y tarjetas con *Ver detalle*, *Guardar* (etiquetas y nota) y
  *Fuente oficial ↗*. Si una fuente falla, aviso y resultados de las demás.
- **Guardados:** filtros por fuente, etiqueta y fechas; cada elemento con etiquetas, nota, fecha de
  guardado y *Borrar*. El número junto a la pestaña es el total.
- **Ayuda:** guía rápida y desplegable de 11 preguntas frecuentes. También se abre con el botón
  *¿Cómo funciona?* de la cabecera.

### 7.2 `html/index.html`

| Elemento | Qué es |
|----------|--------|
| `<script type="module" src="/js/app.js">` | Carga el JavaScript como módulos ES (`import`/`export`) |
| `<nav role="tablist">` | Tres pestañas `role="tab"` con `aria-selected` |
| `<section role="tabpanel">` ×3 | Paneles Buscar, Guardados y Ayuda |
| `data-example="…"` | Botones de ejemplo del buscador |
| `data-open-tab="help"` | Botón *¿Cómo funciona?* |
| `<details name="faq"><summary>…</summary>…</details>` | Cada pregunta frecuente. Se abre y cierra sin JavaScript; `name="faq"` hace que al abrir una se cierre la anterior. Para añadir una pregunta se copia un bloque |
| `<template id="result-template">`, `<template id="saved-template">` | Moldes de las tarjetas, que JavaScript clona |
| `role="status" aria-live="polite"` | Líneas de estado que los lectores de pantalla leen al cambiar |

### 7.3 `css/style.css`

- Colores en variables en `:root`, redefinidas en `@media (prefers-color-scheme: dark)` para el modo
  oscuro automático.
- `--boe` y `--datosgob` con `.badge[data-source="…"]`: color de cada fuente.
- `.faq summary::after` dibuja `+` / `−` en lugar del triángulo nativo.
- `.detail[hidden]` y `.save-form[hidden]` refuerzan `display: none`, porque esos elementos tienen su
  propio `display`.
- `@media (max-width: 560px)`: en móvil, buscador y cabecera se apilan en vertical.

### 7.4 Módulos de JavaScript

```
app.js ──> api.js, state.js, dom.js, tabs.js, search.js, saved.js
search.js ──> api.js, state.js, dom.js, saved.js
saved.js ──> api.js, state.js, dom.js
dom.js ──> state.js
```

| Módulo | Funciones |
|--------|-----------|
| `app.js` | `loadSources()` rellena los dos selectores de fuente; inicializa pestañas, buscador y guardados; recarga la lista al abrir *Guardados*; al arrancar carga las fuentes y luego el contador |
| `tabs.js` | `showTab(name)` marca la pestaña activa y muestra su panel; `onTabShown` registra qué hacer al abrir una; `initTabs` conecta los clics, incluido `data-open-tab` |
| `search.js` | `initSearch` conecta el formulario y los ejemplos (`requestSubmit`). `onSubmit` busca y muestra resultados, avisos o error. `resultCard` crea cada tarjeta; `setupDetail` rellena y despliega el detalle; `setupSave` gestiona el formulario de guardar |
| `saved.js` | `refreshSavedAll` pide todos los guardados para el contador, el filtro de etiquetas y saber qué está guardado. `loadSaved` aplica los filtros y pinta la lista. `fillTagFilter` ordena las etiquetas en español. `savedCard` crea cada tarjeta con su botón de borrar |
| `state.js` | `state.sources` (nombre → etiqueta), `state.savedAll`, `EXTRA_LABELS` (nombres legibles de los campos `extra`), `sourceLabel` e `isSaved` |
| `dom.js` | `$` (atajo de `querySelector`), `setStatus`, `formatDate` (`2007-07-12` → `12/07/2007` sin usar `Date`, para que la zona horaria no cambie el día), `safeURL`, `cloneTemplate`, `fillCard` (lo común a las dos tarjetas) y `plural` |

Seguridad en el navegador: todo el texto se pinta con `textContent` (nunca `innerHTML`), así un
`<script>` dentro de un título se ve como texto y no se ejecuta; y solo se enlazan URLs `http(s)`.

---

## 8. Servicio `proxy`

Código en `srcs/requirements/proxy/src/`. Es la única puerta de entrada.

### `main.go`

| Paso | Qué hace |
|------|----------|
| `-healthcheck` | Pide `https://127.0.0.1:443/healthz` **a través del propio proxy**: solo da OK si el TLS, el proxy y la app funcionan |
| Configuración | `PROXY_PORT` (443), `DOMAIN_NAME` (localhost), `UPSTREAM` (`http://app:8080`), `CERT_DIR` (`/certs`) |
| `httputil.ReverseProxy` | `SetURL(upstream)` cambia el destino; `SetXForwarded()` añade `X-Forwarded-For/Host/Proto`; mantiene la cabecera `Host`; si la app no responde, `502` |
| Log | Una línea por petición: IP, método, ruta y duración |
| `tls.Config{MinVersion: TLS12, MaxVersion: TLS13}` | Rechaza TLS 1.0 y 1.1 |

### `cert.go`

`ensureCert` reutiliza `server.crt` si ya existe en el volumen. Si no, genera una clave ECDSA P-256 y
un certificado X.509 **autofirmado** de un año para `DOMAIN_NAME`, y los guarda en PEM (la clave con
permisos `0600`). Para regenerarlo basta con borrar `~/data/certs`.

### `healthcheck.go`

`runHealthcheck` funciona como `internal/health` de la app. Está duplicado porque el proxy es un
módulo Go independiente y así cada servicio se construye por separado.

---

## 9. Tests

Todos están en `src/tests/`, **separados del código**. Son un paquete propio (`package tests`), así que
solo usan lo que cada paquete exporta, y no necesitan Internet: las APIs públicas se simulan con
servidores HTTP locales.

```sh
make test                                                   # dentro de Docker
cd srcs/requirements/app/src && go test -v ./tests/...      # con Go instalado
```

| Fichero | Qué contiene |
|---------|--------------|
| `helpers_test.go` | `fakeAPI` (servidor local que responde con un fichero de `testdata/` y guarda la última petición), `stubSource` (fuente falsa que puede fallar o tardar), `newTestApp` (API completa con almacén temporal) y `testApp.do` (petición sin abrir puertos) |
| `sources_test.go` | Petición y traducción del BOE; búsqueda por número oficial; limpieza de caracteres; respuesta vacía; error HTTP; datos.gob.es con sus tres formatos de campo; formatos de fecha; `SearchAll` con una fuente que responde, otra que falla y otra que tarda demasiado |
| `store_test.go` | Guardar y recargar; duplicados; orden y filtros; borrar; fichero corrupto; carpeta escribible |
| `api_test.go` | `/api/sources`; búsqueda con error parcial y con todas fallando (`502`); validación de parámetros; ciclo completo de guardados con filtros, persistencia y borrado; nueve cuerpos no válidos; frontend, `404` y `405` |
| `health_config_test.go` | `health.Check` con servidor sano, con `503` y apagado; configuración por defecto, desde el entorno y con un tiempo máximo no válido |

| `testdata/` | Representa |
|-------------|------------|
| `boe_busqueda.json` | Dos normas, una sin enlace HTML y con vigencia agotada |
| `boe_vacio.json` | Búsqueda sin resultados (`"data": {}`) |
| `datosgob_busqueda.json` | Tres conjuntos de datos con los distintos formatos y uno sin `_about` |
| `datosgob_fechas.json` | Fechas en español, no válidas y solo con `issued` |

Los tests decodifican las respuestas en los tipos de `contract.go`, así que detectan cualquier cambio
en el contrato. No cubren las APIs reales ni el JavaScript del navegador.

---

## 10. Docker, Compose, `.env` y Makefile

### 10.1 Dockerfile de la `app`

```
build:  alpine:3.23 + go       →  CGO_ENABLED=0 go build  →  binario estático con web/ incrustado
test:   FROM build             →  go vet ./... && go test ./tests/...   (solo con make test)
final:  alpine:3.23 + ca-certificates  →  solo el binario
```

| Instrucción de la etapa final | Para qué |
|-------------------------------|----------|
| `apk add ca-certificates` | Sin ellos Go no puede verificar el HTTPS del BOE ni de datos.gob.es |
| `adduser -D -H app` + `chown /data` | Usuario sin privilegios dueño de la carpeta de datos |
| `COPY --from=build` | Solo el binario: ni compilador ni código fuente |
| `USER app` | No se ejecuta como root |
| `HEALTHCHECK … app -healthcheck` | Cada 10 s; 3 s de límite; 5 s de margen al arrancar; 3 fallos → `unhealthy` |
| `ENTRYPOINT` | El binario es el PID 1 y recibe bien la señal de `docker stop` |

El Dockerfile del **proxy** sigue el mismo patrón sin etapa de test, crea `/certs` y expone el 443.

### 10.2 `docker-compose.yml`

| Elemento | Configuración |
|----------|---------------|
| `app` | Volumen `app_data:/data`, red `cloudfgd`, sin puertos publicados, `restart: unless-stopped` |
| `proxy` | Publica `443:443`, volumen `certs_data:/certs`, arranca cuando la `app` está `healthy` (`depends_on … service_healthy`) |
| Volúmenes | `app_data` → `~/data/app` (`saved.json`); `certs_data` → `~/data/certs`. Son *bind* a carpetas que crea el Makefile |
| Red | `cloudfgd` (bridge): los contenedores se ven por su nombre, por eso el proxy usa `http://app:8080` |

### 10.3 `srcs/.env`

| Variable | Valor | Uso |
|----------|-------|-----|
| `DOMAIN_NAME` | `localhost` | Nombre del certificado |
| `APP_PORT` | `8080` | Puerto interno de la app |
| `UPSTREAM` | `http://app:8080` | Destino del proxy |
| `SEARCH_TIMEOUT` | `10s` | Tiempo máximo de cada API externa |

Opcionales: `DATA_PATH`, `BOE_BASE_URL`, `DATOSGOB_BASE_URL`, `PROXY_PORT`, `CERT_DIR`.

### 10.4 Makefile

| Regla | Qué hace |
|-------|----------|
| `make` / `make up` | Crea `~/data/app` y `~/data/certs`, construye y arranca en segundo plano |
| `make down` | Para y elimina los contenedores (los datos se conservan) |
| `make stop` / `make start` | Pausa / reanuda |
| `make logs` | Logs en directo, incluido qué fuente falla y por qué |
| `make ps` | Estado de los contenedores |
| `make health` | Estado del healthcheck de `app` y `proxy` |
| `make test` | `go vet` y tests dentro de Docker (no hace falta Go instalado) |
| `make clean` | `down` + `docker system prune -af` |
| `make fclean` | Borra contenedores, volúmenes, imágenes **y los datos de `~/data`** |
| `make re` | `fclean` + `all` |

Puesta en marcha:

```sh
make
make health
```

Después se abre **https://localhost** y se acepta el aviso del certificado autofirmado.

---

## 11. API REST

```sh
curl -k https://localhost/api/sources
curl -k "https://localhost/api/search?q=trabajo%20aut%C3%B3nomo&source=boe"
curl -k https://localhost/api/saved
curl -k -X POST https://localhost/api/saved -H 'Content-Type: application/json' \
     -d '{"source":"boe","external_id":"BOE-A-2007-13409","title":"Ley 20/2007","url":"https://www.boe.es/buscar/act.php?id=BOE-A-2007-13409","tags":["autónomos"]}'
curl -k -X DELETE https://localhost/api/saved/<id>
```

| Método | Ruta | Respuestas |
|--------|------|------------|
| `GET` | `/api/sources` | `200` + `[{name, label}]` |
| `GET` | `/api/search?q=&source=&limit=` | `200` + `{query, results, errors}` · `400` parámetros no válidos · `502` si fallan todas las fuentes |
| `GET` | `/api/saved?source=&tag=&from=&to=` | `200` + lista · `400` filtros no válidos |
| `POST` | `/api/saved` | `201` + elemento · `400` datos no válidos · `409` ya guardado (con `id`) |
| `DELETE` | `/api/saved/{id}` | `204` · `404` si no existe |
| `GET` | `/healthz` | `200 ok` · `503` si el volumen no admite escrituras |

Cualquier error de la API responde `{"error": "mensaje"}`.

---

## 12. Problemas frecuentes

- **`proxy` se queda en `Created`:** espera a que la `app` esté `healthy`. Revisa `make logs` y `make health`.
- **`app` en `unhealthy`:** `~/data/app` no existe o no admite escrituras. Ejecuta `make` o revisa permisos.
- **"La fuente tardó demasiado en responder":** la API externa va lenta. Reintenta más tarde o sube
  `SEARCH_TIMEOUT` en `srcs/.env` y ejecuta `make down && make` (no `make re`, que borra los guardados).
- **"La fuente externa no respondió correctamente":** `make logs` muestra el código HTTP de la API.
- **Todas las búsquedas fallan:** el contenedor no tiene salida a Internet o DNS. Prueba
  `docker exec app nslookup www.boe.es`.
- **`x509: certificate signed by unknown authority`:** falta `ca-certificates`; reconstruye con `make re`.
- **`502 Bad Gateway` en todas las páginas:** el proxy no llega a la app; comprueba la red con
  `docker network inspect srcs_cloudfgd`.
- **Falla `make test`:** `cd srcs/requirements/app/src && go test -v ./tests/...` muestra qué test y por qué.
- **Aviso de certificado en el navegador:** es normal, el certificado es autofirmado.

---

## 13. Referencias de las herramientas

Documentación oficial de lo que usa el código, por si una parte no se entiende.

| Tema | Para qué se usa aquí | Documentación |
|------|----------------------|---------------|
| GNU Make | `Makefile` | [Manual de GNU Make](https://www.gnu.org/software/make/manual/make.html) |
| Dockerfile | Imágenes de `app` y `proxy` | [Dockerfile reference](https://docs.docker.com/reference/dockerfile/) · [Multi-stage builds](https://docs.docker.com/build/building/multi-stage/) |
| Docker Compose | Servicios, red y volúmenes | [Services](https://docs.docker.com/reference/compose-file/services/) · [Networking](https://docs.docker.com/compose/how-tos/networking/) · [Volumes](https://docs.docker.com/engine/storage/volumes/) |
| Go | Lenguaje | [Tour de Go](https://go.dev/tour/) · [Go by Example](https://gobyexample.com/) · [Librería estándar](https://pkg.go.dev/std) |
| `net/http` y rutas con método | Servidor y handlers | [net/http](https://pkg.go.dev/net/http) · [Routing enhancements](https://go.dev/blog/routing-enhancements) |
| `embed` | Frontend dentro del binario | [embed](https://pkg.go.dev/embed) |
| `encoding/json` y `json.RawMessage` | JSON propio y de las APIs | [encoding/json](https://pkg.go.dev/encoding/json) |
| `sync.Mutex`, goroutines, `WaitGroup` | Almacén y búsqueda en paralelo | [sync](https://pkg.go.dev/sync) · [Go by Example: WaitGroups](https://gobyexample.com/waitgroups) |
| `context` y `http.Client` | Tiempo máximo de las APIs | [context](https://pkg.go.dev/context) · [http.Client](https://pkg.go.dev/net/http#Client) |
| `testing` y `httptest` | Tests con APIs simuladas | [testing](https://pkg.go.dev/testing) · [httptest](https://pkg.go.dev/net/http/httptest) |
| TLS y proxy inverso | Servicio `proxy` | [crypto/tls](https://pkg.go.dev/crypto/tls) · [httputil.ReverseProxy](https://pkg.go.dev/net/http/httputil#ReverseProxy) · [¿Qué es TLS?](https://www.cloudflare.com/es-es/learning/ssl/transport-layer-security-tls/) |
| HTML `<template>` y `<details>` | Tarjetas y preguntas frecuentes | [`<template>`](https://developer.mozilla.org/es/docs/Web/HTML/Element/template) · [`<details>`](https://developer.mozilla.org/es/docs/Web/HTML/Reference/Elements/details) |
| Fetch y módulos ES | `api.js` y el resto de módulos | [Uso de Fetch](https://developer.mozilla.org/es/docs/Web/API/Fetch_API/Using_Fetch) · [Módulos de JavaScript](https://developer.mozilla.org/es/docs/Web/JavaScript/Guide/Modules) |
| `textContent` y XSS | Pintar texto de forma segura | [Node.textContent](https://developer.mozilla.org/es/docs/Web/API/Node/textContent) · [OWASP XSS](https://cheatsheetseries.owasp.org/cheatsheets/Cross_Site_Scripting_Prevention_Cheat_Sheet.html) |
| API del BOE | `boe.go` | [Datos abiertos del BOE](https://www.boe.es/datosabiertos/api/api.php) · [Manual (PDF)](https://www.boe.es/datosabiertos/documentos/APIconsolidada.pdf) |
| API de datos.gob.es | `datosgob.go` | [apidata](https://datos.gob.es/en/accessible-apidata) |
