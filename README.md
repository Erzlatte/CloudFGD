# CloudFGD

CloudFGD nace como un proyecto integral de infraestructura y DevOps diseñado para cumplir con los estándares de alta disponibilidad y eficiencia que exige el mercado actual. El objetivo principal es diseñar y desplegar un entorno completo que abarque todo el ciclo de vida del desarrollo de software, desde la preparación de los entornos de trabajo hasta la monitorización en producción.

Estado actual y tecnología base El proyecto cuenta actualmente con una base sólida montada mediante Docker y Docker Compose, que orquesta una aplicación web desarrollada en Go, garantizando un entorno ligero, rápido y consistente para el desarrollo.

Visión a futuro CloudFGD no es solo una aplicación web, sino una demostración práctica de las mejores prácticas DevOps. El sistema se estructurará en cuatro pilares fundamentales que reflejan la colaboración esencial entre equipos de desarrollo y operaciones (DevOps):

Infraestructura: Preparación y gestión de entornos de desarrollo escalables.
Despliegue: Automatización de procesos para lanzamientos rápidos y seguros.
Monitorización: Implementación de supervisión constante para el mantenimiento de la disponibilidad.
Documentación: Centralización del conocimiento para asegurar la trazabilidad y el mantenimiento del sistema.

A través de CloudFGD, buscaremos demostrar cómo la coordinación de estas áreas permite entregar servicios robustos y mantenibles, integrando herramientas modernas para la automatización y la supervisión continua.

---

## Producto 1: archivador de información pública

La aplicación es una web con **API REST propia** que consulta APIs públicas y deja guardar lo que
cada usuario quiere seguir:

1. **Buscar** en varias fuentes desde un único buscador (hoy: BOE y datos.gob.es).
2. **Ver el detalle** de un resultado: título, fecha, resumen, datos de la fuente y enlace oficial.
3. **Guardar** lo que interesa, con etiquetas y una nota personal.
4. **Consultar la lista de guardados**, filtrarla por fuente, etiqueta o fecha, y borrar elementos.

La web no sustituye a las fuentes oficiales: las consulta, normaliza lo que devuelven y guarda una
copia ligera de lo marcado, siempre con el enlace a la publicación original.

| Documento | Contenido |
|-----------|-----------|
| [docs/API.md](docs/API.md) | Referencia de la API REST propia: rutas, parámetros, respuestas y errores |
| [docs/CONECTORES.md](docs/CONECTORES.md) | Cómo se consulta cada API pública y cómo añadir una fuente nueva |
| [docs/FRONTEND.md](docs/FRONTEND.md) | Pantallas, ficheros y funcionamiento de la interfaz |
| [CHANGELOG.md](CHANGELOG.md) | Qué ha cambiado respecto al libro de visitas |
| [DEVOPS_AWS_K8S.md](DEVOPS_AWS_K8S.md) | Plan opcional para llevar el proyecto a AWS con Kubernetes |

---

## Índice

1. [Arquitectura](#1-arquitectura)
2. [Estructura del proyecto](#2-estructura-del-proyecto)
3. [Cómo funciona una petición](#3-cómo-funciona-una-petición)
4. [El servicio `proxy`](#4-el-servicio-proxy)
5. [El servicio `app`](#5-el-servicio-app)
6. [El frontend](#6-el-frontend)
7. [Docker: imágenes, compose y volúmenes](#7-docker-imágenes-compose-y-volúmenes)
8. [Health checks](#8-health-checks)
9. [Uso y comandos del Makefile](#9-uso-y-comandos-del-makefile)
10. [API](#10-api)
11. [Problemas frecuentes](#11-problemas-frecuentes)
12. [Documentación de las herramientas](#12-documentación-de-las-herramientas)

---

## 1. Arquitectura

```
                 Host                                Red Docker "cloudfgd"                          Internet
 ┌─────────────────────────┐       ┌────────────────────────────────────────────────┐
 │                         │  443  │  ┌─────────────┐  HTTP  ┌──────────────────────┐ │  HTTPS  ┌──────────────┐
 │  Navegador  ────────────┼───────┼─>│   proxy     │───────>│        app           │─┼────────>│ API del BOE  │
 │  https://localhost      │  TLS  │  │ (Go, :443)  │ :8080  │     (Go, :8080)      │ │         └──────────────┘
 │                         │       │  └──────┬──────┘        │ API REST + conectores│─┼────────>┌──────────────┐
 └─────────────────────────┘       │         │ /certs        └──────────┬───────────┘ │         │ datos.gob.es │
                                   └─────────┼──────────────────────────┼─────────────┘         └──────────────┘
                                             ▼                          ▼ /data
                                       ~/data/certs             ~/data/app/saved.json
```

| Servicio | Puerto | Expuesto al host | Responsabilidad |
|----------|--------|------------------|-----------------|
| `proxy`  | 443    | **Sí** (único)   | Termina TLS, genera el certificado, reenvía peticiones a `app`, registra accesos |
| `app`    | 8080   | No               | Sirve el frontend, expone la API REST, consulta las APIs públicas y guarda los elementos de interés |

El navegador **solo habla con el proxy**, y la `app` es la única pieza que llama a las fuentes públicas
y al almacén de guardados. Así el navegador nunca depende directamente de una API externa.

---

## 2. Estructura del proyecto

```
CloudFGD/
├── Makefile                         # make, make down, make health, make test...
├── README.md
├── CHANGELOG.md                     # Cambios de cada versión
├── DEVOPS_AWS_K8S.md                # Plan opcional AWS + Kubernetes
├── docs/
│   ├── API.md                       # Referencia de la API REST propia
│   ├── CONECTORES.md                # Conectores a las APIs públicas
│   └── FRONTEND.md                  # Interfaz web
└── srcs/
    ├── .env                         # Variables de entorno (dominio, puertos, timeout)
    ├── docker-compose.yml
    └── requirements/
        ├── proxy/                   # ── Servicio 1: proxy inverso TLS (sin cambios) ──
        │   ├── Dockerfile
        │   └── src/
        │       ├── main.go
        │       ├── cert.go
        │       └── healthcheck.go
        └── app/                     # ── Servicio 2: aplicación ──
            ├── Dockerfile           # Incluye una etapa "test" para make test
            └── src/
                ├── go.mod
                ├── main.go          # Arranque, configuración y rutas
                ├── handlers.go      # Handlers de la API REST y /healthz
                ├── handlers_test.go # Tests de la API (sin red)
                ├── store.go         # Guardados: JSON con mutex y escritura atómica
                ├── healthcheck.go
                ├── sources/         # Conectores a las APIs públicas
                │   ├── source.go    # Interfaz común Source y formato Result
                │   ├── search.go    # Búsqueda en varias fuentes a la vez
                │   ├── boe.go       # Conector del BOE
                │   ├── datosgob.go  # Conector de datos.gob.es
                │   ├── sources_test.go
                │   └── testdata/    # Respuestas de ejemplo para los tests
                └── web/             # Frontend (se incrusta en el binario)
                    ├── html/index.html
                    ├── css/style.css
                    └── js/app.js
```

Ambos servicios siguen usando **solo la librería estándar de Go**: no hay dependencias externas.

---

## 3. Cómo funciona una petición

**Buscar**

1. El usuario escribe "trabajo autónomo", elige la fuente (o "Todas") y pulsa *Buscar*.
2. `app.js` llama a `GET /api/search?q=trabajo autónomo&source=all`.
3. La `app` lanza **una consulta por fuente en paralelo** (una goroutine por conector), cada una con
   un tiempo máximo (`SEARCH_TIMEOUT`).
4. Cada conector traduce la respuesta de su API al formato común `Result`
   (fuente, identificador, título, resumen, fecha, enlace oficial y datos extra).
5. La `app` devuelve todos los resultados juntos y, aparte, la lista de fuentes que fallaron.
   Si una API externa no responde, **las demás se muestran igualmente**.

**Guardar**

1. En un resultado, el usuario pulsa *Guardar*, escribe etiquetas y una nota, y confirma.
2. `app.js` envía el resultado con las etiquetas y la nota a `POST /api/saved`.
3. La `app` valida los datos (fuente conocida, enlace `http(s)`, longitudes, formato de fecha),
   normaliza las etiquetas y lo añade a `/data/saved.json` con escritura atómica.
4. Si ya estaba guardado (misma fuente e identificador), responde `409 Conflict`.

**Consultar guardados**

1. En la pestaña *Guardados*, cada cambio en los filtros llama a `GET /api/saved?source=&tag=&from=&to=`.
2. La `app` filtra en memoria y devuelve la lista, del más reciente al más antiguo.
3. *Borrar* llama a `DELETE /api/saved/{id}`.

---

## 4. El servicio `proxy`

**Sin cambios** respecto a la versión anterior. Está en `srcs/requirements/proxy/src/`.

- **`main.go`:** TLS 1.2/1.3, proxy inverso con `httputil.ReverseProxy`, cabeceras `X-Forwarded-*`,
  `502 Bad Gateway` si la `app` no responde, una línea de log por petición y modo `-healthcheck`.
- **`cert.go`:** genera un certificado autofirmado ECDSA P-256 la primera vez y lo guarda en `/certs`.

---

## 5. El servicio `app`

Está en `srcs/requirements/app/src/`. El detalle de cada ruta está en [docs/API.md](docs/API.md) y el
de los conectores en [docs/CONECTORES.md](docs/CONECTORES.md).

### `main.go` — arranque y rutas

- Lee la configuración del entorno (`APP_PORT`, `DATA_PATH`, `SEARCH_TIMEOUT` y, solo para pruebas,
  `BOE_BASE_URL` y `DATOSGOB_BASE_URL`).
- Crea los conectores y los registra en `App.sources`. **Añadir una fuente es añadir una línea aquí.**
- Define las rutas:

  | Ruta | Handler | Qué hace |
  |------|---------|----------|
  | `GET /` | inline | Sirve `web/html/index.html` |
  | `GET /css/…`, `GET /js/…` | `http.FileServer` | Ficheros estáticos |
  | `GET /api/sources` | `listSources` | Fuentes disponibles |
  | `GET /api/search` | `search` | Busca en una fuente o en todas |
  | `GET /api/saved` | `listSaved` | Lista de guardados con filtros |
  | `POST /api/saved` | `createSaved` | Guarda un elemento |
  | `DELETE /api/saved/{id}` | `deleteSaved` | Borra un elemento |
  | `GET /healthz` | `healthz` | Estado del servicio |

### `sources/` — conectores

- `source.go` define la interfaz que cumple cada conector:

  ```go
  type Source interface {
      Name() string  // "boe", "datosgob": se usa en ?source=
      Label() string // nombre legible para el frontend
      Search(ctx context.Context, query string, limit int) ([]Result, error)
  }
  ```

- `boe.go` busca en la **legislación consolidada del BOE** por palabras del título, o por número
  oficial si se escribe algo como `39/2015`.
- `datosgob.go` busca **conjuntos de datos de datos.gob.es** por título, y entiende los distintos
  formatos en que esa API devuelve un mismo campo (texto, objeto multiidioma o lista).
- `search.go` (`SearchAll`) consulta varias fuentes en paralelo y separa resultados y errores.

### `store.go` — guardados

- Mismo diseño que tenía el libro de visitas: lista en memoria, `sync.Mutex` y escritura atómica
  (fichero temporal + `os.Rename`), ahora en `/data/saved.json`.
- Cada `SavedItem` es una copia ligera del resultado más `tags`, `note`, `id` y `saved_at`.
- `List(Filter)` filtra por fuente, etiqueta y rango de fechas del documento.

### `handlers.go` — API REST

- Valida todo lo que llega del navegador: longitudes, fechas `AAAA-MM-DD`, fuentes conocidas y
  enlaces solo `http(s)` (así no se puede guardar un `javascript:` que luego se pinte en un enlace).
- Normaliza las etiquetas: minúsculas, sin espacios sobrantes, sin repetidas, máximo 10.
- Responde siempre JSON, con los errores en el formato `{"error": "..."}`.

### Tests

- `sources/sources_test.go`: cada conector se prueba contra un **servidor HTTP falso** (`httptest`)
  que devuelve respuestas de ejemplo de `testdata/`. Comprueba la petición que se envía y cómo se
  traduce la respuesta. No necesita Internet.
- `handlers_test.go`: prueba la API completa (búsqueda, errores parciales, guardar, duplicados,
  filtros, borrar, validaciones y que los datos sobreviven a un reinicio).

---

## 6. El frontend

Separado en `web/html`, `web/css` y `web/js`, sin frameworks. Detalle completo en
[docs/FRONTEND.md](docs/FRONTEND.md).

- **Pestaña Buscar:** texto, selector de fuente y resultados en tarjetas con la fuente, la fecha,
  el título y un resumen. Cada tarjeta tiene *Ver detalle*, *Guardar* (con etiquetas y nota) y el
  enlace a la *Fuente oficial*. Si una fuente falla, aparece un aviso y se muestran las demás.
- **Pestaña Guardados:** filtros por fuente, etiqueta y fechas; cada elemento muestra sus etiquetas,
  su nota, la fecha en que se guardó y un botón para borrarlo.
- Todo el texto se pinta con `textContent` (protección contra XSS) y solo se enlazan URLs `http(s)`.
- Modo oscuro automático y diseño adaptado a móvil.
- Pie de página con el aviso de que, para usos jurídicos, hay que consultar la publicación oficial.

---

## 7. Docker: imágenes, compose y volúmenes

### Dockerfiles (multi-stage)

```
Etapa build:  alpine:3.23 + paquete go  →  go build  →  binario estático
Etapa test:   (solo con make test)      →  go vet + go test
Etapa final:  alpine:3.23 + ca-certificates  →  solo se copia el binario
```

- **Nuevo:** la imagen final de la `app` instala `ca-certificates`, necesario para que Go pueda
  verificar los certificados HTTPS del BOE y de datos.gob.es.
- **Nuevo:** la etapa `test` solo se ejecuta con `make test`; un `make` normal no pasa por ella.
- La `app` se ejecuta con un usuario sin privilegios y el binario es el PID 1 del contenedor.

### `docker-compose.yml`

Sin cambios: red `cloudfgd`, solo el puerto 443 publicado, `depends_on` con `service_healthy`,
`restart: unless-stopped` y volúmenes en `~/data`. Los contenedores pueden salir a Internet por
defecto, así que no hace falta configurar nada para llegar a las APIs públicas.

| Volumen | Contenedor | Host | Contenido |
|---------|------------|------|-----------|
| `app_data`   | `/data`  | `~/data/app`   | `saved.json` (guardados) |
| `certs_data` | `/certs` | `~/data/certs` | `server.crt`, `server.key` |

### `.env`

| Variable | Valor por defecto | Uso |
|----------|-------------------|-----|
| `DOMAIN_NAME` | `localhost` | CN del certificado |
| `APP_PORT` | `8080` | Puerto interno de la `app` |
| `UPSTREAM` | `http://app:8080` | Destino del `proxy` |
| `SEARCH_TIMEOUT` | `10s` | **Nuevo.** Tiempo máximo de espera de cada API externa |
| `BOE_BASE_URL`, `DATOSGOB_BASE_URL` | URLs oficiales | Opcionales, solo para apuntar a una API simulada en pruebas |

---

## 8. Health checks

Sin cambios: cada Dockerfile tiene un `HEALTHCHECK` que ejecuta el propio binario con `-healthcheck`.

| Servicio | Qué comprueba | Se marca *unhealthy* si… |
|----------|---------------|--------------------------|
| `app`   | `GET http://127.0.0.1:8080/healthz` | el servidor no responde o `/data` no es escribible |
| `proxy` | `GET https://127.0.0.1:443/healthz` | falla el TLS, el proxy no responde o la `app` no está sana |

`/healthz` **no** consulta las APIs externas a propósito: si el BOE se cae, la aplicación sigue
sana (puede seguir mostrando los guardados y buscando en las demás fuentes).

---

## 9. Uso y comandos del Makefile

Requisitos: Docker con el plugin `docker compose`, `make` y conexión a Internet (para las APIs públicas).

```sh
make            # crea ~/data/{app,certs}, construye las imágenes y levanta los servicios
```

Abre **https://localhost**. El certificado es autofirmado, así que el navegador mostrará un aviso.

| Comando | Qué hace |
|---------|----------|
| `make` / `make up` | Construye y arranca en segundo plano |
| `make down` | Para y elimina los contenedores (los datos se conservan) |
| `make stop` / `make start` | Pausa / reanuda los contenedores |
| `make logs` | Logs en directo (incluye qué fuente falla y por qué) |
| `make ps` | Contenedores y su estado |
| `make health` | Estado del health check de cada contenedor |
| `make test` | **Nuevo.** `go vet` y tests de la `app` dentro de Docker (no hace falta Go instalado) |
| `make clean` | `down` + `docker system prune -af` |
| `make fclean` | Elimina contenedores, volúmenes, imágenes **y los datos de `~/data`** |
| `make re` | `fclean` + `all` |

Con Go instalado también se pueden lanzar los tests directamente:

```sh
cd srcs/requirements/app/src && go test ./...
```

---

## 10. API

Resumen de la API REST propia. La referencia completa, con ejemplos de respuesta y todos los
errores, está en [docs/API.md](docs/API.md).

```sh
curl -k https://localhost/api/sources
curl -k "https://localhost/api/search?q=trabajo%20aut%C3%B3nomo&source=boe"
curl -k https://localhost/api/saved
curl -k -X DELETE https://localhost/api/saved/<id>
```

| Método | Ruta | Respuesta |
|--------|------|-----------|
| `GET` | `/api/sources` | `200` + lista de fuentes `[{name, label}]` |
| `GET` | `/api/search?q=&source=&limit=` | `200` + `{query, results, errors}` · `400` parámetros no válidos · `502` si fallan todas las fuentes |
| `GET` | `/api/saved?source=&tag=&from=&to=` | `200` + lista de guardados · `400` filtros no válidos |
| `POST` | `/api/saved` | `201` + elemento creado · `400` datos no válidos · `409` ya guardado |
| `DELETE` | `/api/saved/{id}` | `204` · `404` si no existe |
| `GET` | `/healthz` | `200 ok` · `503` si el volumen no es escribible |

---

## 11. Problemas frecuentes

- **`proxy` no arranca / se queda en `Created`:** espera a que la `app` esté *healthy*.
  Revisa `make logs` y `make health`.
- **`app` en estado `unhealthy`:** el volumen `~/data/app` no existe o no tiene permisos de
  escritura. Ejecuta `make` (que crea las carpetas) o revisa los permisos.
- **Aviso "la fuente tardó demasiado en responder":** la API externa va lenta. Vuelve a intentarlo
  más tarde, o sube `SEARCH_TIMEOUT` en `srcs/.env` (por ejemplo, `20s`) y ejecuta
  `make down && make` (no uses `make re`, que borra los guardados).
- **Aviso "la fuente externa no respondió correctamente":** la API devolvió un error. `make logs`
  muestra el código HTTP. Si es `400` en el BOE, la consulta tenía una sintaxis que la API no admite.
- **Todas las búsquedas fallan:** el contenedor no tiene salida a Internet o no resuelve DNS.
  Comprueba la resolución con `docker exec app nslookup www.boe.es` y mira el error exacto en `make logs`.
- **Error de certificado al llamar a las APIs (`x509: certificate signed by unknown authority`):**
  falta `ca-certificates` en la imagen; reconstruye con `make re`.
- **`502 Bad Gateway` en todas las páginas:** el `proxy` no llega a la `app`. Comprueba que ambos
  contenedores están en la red `cloudfgd` (`docker network inspect srcs_cloudfgd`).
- **Los mensajes del libro de visitas han desaparecido:** la versión actual ya no los usa; el
  fichero antiguo `~/data/app/messages.json` sigue en disco y se puede borrar.
- **Aviso de certificado en el navegador:** es normal, el certificado es autofirmado.

---

## 12. Documentación de las herramientas

Esta sección recoge la documentación oficial de cada herramienta que usa el proyecto, **ordenada
según el flujo de trabajo**: desde que se ejecuta `make` hasta que el navegador muestra un resultado.
No hace falta leerlo todo; cada tabla indica *para qué* se usa esa pieza en CloudFGD, así que
puedes ir directamente a la parte que no entiendas.

```
 make ──> docker compose ──> Dockerfile (Alpine + Go) ──> contenedores + red + volúmenes
                                                              │
     navegador ──HTTPS──> proxy (Go: TLS + reverse proxy) ──> app (Go: API + JSON en disco)
                                                              │
                                              frontend (HTML + CSS + JavaScript)
```

### 12.1 Punto de partida: Make

| Herramienta | Para qué se usa aquí | Documentación |
|-------------|----------------------|---------------|
| GNU Make | `Makefile` con los comandos `make`, `make down`, `make health`… | [Manual de GNU Make](https://www.gnu.org/software/make/manual/make.html) |
| Reglas `.PHONY` | Marca las reglas que no generan ficheros | [Phony Targets](https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html) |

### 12.2 Contenedores: Docker

| Herramienta | Para qué se usa aquí | Documentación |
|-------------|----------------------|---------------|
| Docker (conceptos) | Qué son imágenes, contenedores y el daemon | [Docker overview](https://docs.docker.com/get-started/docker-overview/) |
| Dockerfile | Receta para construir las imágenes de `proxy` y `app` | [Dockerfile reference](https://docs.docker.com/reference/dockerfile/) |
| Multi-stage build | Compilar en una etapa y copiar solo el binario a la final | [Multi-stage builds](https://docs.docker.com/build/building/multi-stage/) |
| `HEALTHCHECK` | Comprobar que cada contenedor está sano | [HEALTHCHECK](https://docs.docker.com/reference/dockerfile/#healthcheck) |
| `ENTRYPOINT` | Lanzar el binario de Go como PID 1 | [ENTRYPOINT](https://docs.docker.com/reference/dockerfile/#entrypoint) |
| `USER` | Ejecutar la `app` sin privilegios de root | [USER](https://docs.docker.com/reference/dockerfile/#user) |
| Alpine Linux | Imagen base ligera de ambos servicios | [Alpine en Docker Hub](https://hub.docker.com/_/alpine) · [Paquetes de Alpine](https://pkgs.alpinelinux.org/packages) |

### 12.3 Orquestación: Docker Compose

| Herramienta | Para qué se usa aquí | Documentación |
|-------------|----------------------|---------------|
| `docker compose` (CLI) | `up`, `down`, `logs`, `ps`… que llama el Makefile | [docker compose CLI](https://docs.docker.com/reference/cli/docker/compose/) |
| Servicios | Definición de `app` y `proxy` en `docker-compose.yml` | [Services](https://docs.docker.com/reference/compose-file/services/) |
| `depends_on` | El `proxy` espera a que la `app` esté *healthy* | [depends_on](https://docs.docker.com/reference/compose-file/services/#depends_on) |
| Redes | Red `cloudfgd` para que los servicios se encuentren por nombre | [Networking in Compose](https://docs.docker.com/compose/how-tos/networking/) |
| Volúmenes | Persistencia de `saved.json` y del certificado | [Volumes](https://docs.docker.com/engine/storage/volumes/) · [Bind mounts](https://docs.docker.com/engine/storage/bind-mounts/) |
| Variables y `.env` | `--env-file srcs/.env` y `${DATA_DIR}` en el compose | [Variables e interpolación](https://docs.docker.com/compose/how-tos/environment-variables/variable-interpolation/) |
| `restart` | `unless-stopped`: reinicio automático si un contenedor cae | [Restart policies](https://docs.docker.com/engine/containers/start-containers-automatically/) |

### 12.4 El lenguaje: Go

| Recurso | Para qué sirve | Enlace |
|---------|----------------|--------|
| Documentación oficial | Instalación, guías y referencia del lenguaje | [go.dev/doc](https://go.dev/doc/) |
| Tour de Go | Tutorial interactivo para aprender la sintaxis desde cero | [A Tour of Go](https://go.dev/tour/) |
| Go by Example | Ejemplos cortos y comentados de cada concepto | [gobyexample.com](https://gobyexample.com/) |
| Librería estándar | Referencia de todos los paquetes que usa el proyecto | [pkg.go.dev/std](https://pkg.go.dev/std) |
| Módulos (`go.mod`) | Qué define el fichero `go.mod` de cada servicio | [go.mod reference](https://go.dev/doc/modules/gomod-ref) |

### 12.5 Servicio `proxy` (TLS y proxy inverso)

| Concepto / paquete | Para qué se usa aquí | Documentación |
|--------------------|----------------------|---------------|
| Qué es TLS/HTTPS | El cifrado que protege todo el tráfico de entrada | [¿Qué es TLS? (Cloudflare)](https://www.cloudflare.com/es-es/learning/ssl/transport-layer-security-tls/) |
| Qué es un proxy inverso | El papel que cumple el servicio `proxy` | [Proxy inverso (Cloudflare)](https://www.cloudflare.com/es-es/learning/cdn/glossary/reverse-proxy/) |
| `crypto/tls` | Limitar a TLS 1.2 y 1.3 | [pkg.go.dev/crypto/tls](https://pkg.go.dev/crypto/tls) |
| `crypto/x509`, `crypto/ecdsa` | Generar el certificado autofirmado (`cert.go`) | [crypto/x509](https://pkg.go.dev/crypto/x509) · [crypto/ecdsa](https://pkg.go.dev/crypto/ecdsa) |
| `net/http/httputil` | `ReverseProxy`, que reenvía las peticiones a la `app` | [httputil.ReverseProxy](https://pkg.go.dev/net/http/httputil#ReverseProxy) |
| `flag` | El modo `-healthcheck` del binario | [pkg.go.dev/flag](https://pkg.go.dev/flag) |

### 12.6 Servicio `app` (servidor web y datos)

| Concepto / paquete | Para qué se usa aquí | Documentación |
|--------------------|----------------------|---------------|
| `net/http` | Servidor HTTP, rutas y handlers | [pkg.go.dev/net/http](https://pkg.go.dev/net/http) · [Go by Example: HTTP server](https://gobyexample.com/http-server) |
| Rutas con método (Go 1.22+) | Patrones como `DELETE /api/saved/{id}` y `GET /{$}` | [Routing enhancements (blog de Go)](https://go.dev/blog/routing-enhancements) |
| `embed` | Incrustar la carpeta `web/` dentro del binario | [pkg.go.dev/embed](https://pkg.go.dev/embed) · [Go by Example: embed](https://gobyexample.com/embed-directive) |
| `encoding/json` | Leer y escribir `saved.json` , la API propia y las respuestas de las APIs públicas | [pkg.go.dev/encoding/json](https://pkg.go.dev/encoding/json) · [Go by Example: JSON](https://gobyexample.com/json) |
| `sync.Mutex` | Evitar que dos escrituras simultáneas se pisen | [sync.Mutex](https://pkg.go.dev/sync#Mutex) · [Go by Example: mutexes](https://gobyexample.com/mutexes) |
| `os.Rename` | Escritura atómica del fichero de datos | [os.Rename](https://pkg.go.dev/os#Rename) |
| Formato JSON | Formato de los datos guardados y de la API | [json.org (español)](https://www.json.org/json-es.html) |

### 12.7 Frontend (HTML, CSS y JavaScript)

| Concepto | Para qué se usa aquí | Documentación (MDN) |
|----------|----------------------|---------------------|
| HTML | Estructura de `index.html` | [HTML](https://developer.mozilla.org/es/docs/Web/HTML) |
| `<template>` | Molde de cada resultado y de cada guardado que clona `app.js` | [`<template>`](https://developer.mozilla.org/es/docs/Web/HTML/Element/template) |
| Variables CSS | Colores definidos en `:root` | [Propiedades personalizadas](https://developer.mozilla.org/es/docs/Web/CSS/Using_CSS_custom_properties) |
| `prefers-color-scheme` | Modo oscuro automático | [prefers-color-scheme](https://developer.mozilla.org/en-US/docs/Web/CSS/@media/prefers-color-scheme) (inglés) |
| Fetch API | Buscar, guardar y borrar sin recargar la página | [Uso de Fetch](https://developer.mozilla.org/es/docs/Web/API/Fetch_API/Using_Fetch) |
| `textContent` | Pintar texto de forma segura (sin ejecutar HTML) | [Node.textContent](https://developer.mozilla.org/es/docs/Web/API/Node/textContent) |
| `Intl.DateTimeFormat` | Formato de las fechas en español | [Intl.DateTimeFormat](https://developer.mozilla.org/es/docs/Web/JavaScript/Reference/Global_Objects/Intl/DateTimeFormat) |

### 12.8 Conceptos web generales

| Concepto | Dónde aparece en el proyecto | Documentación |
|----------|------------------------------|---------------|
| Códigos de estado HTTP | `200`, `201`, `204`, `400`, `404`, `409`, `502`, `503` de la API y el proxy | [Códigos de estado HTTP (MDN)](https://developer.mozilla.org/es/docs/Web/HTTP/Status) |
| XSS | Por qué el frontend usa `textContent` y no `innerHTML` | [OWASP XSS Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross_Site_Scripting_Prevention_Cheat_Sheet.html) (inglés) |

### 12.9 Orden de lectura recomendado

Si partes de cero, este recorrido sigue el mismo camino que el proyecto:

1. **Docker overview** → entender qué es un contenedor y una imagen.
2. **Dockerfile reference** + **Multi-stage builds** → leer los dos `Dockerfile`.
3. **Docker Compose: Services, Networking y Volumes** → leer `docker-compose.yml`.
4. **Manual de Make** → leer el `Makefile` y ver qué comando de compose lanza cada regla.
5. **Tour de Go** → lo justo para leer sintaxis (funciones, structs, métodos, errores).
6. **net/http** + **Routing enhancements** → leer `app/src/main.go` y `handlers.go`.
7. **encoding/json** + **sync.Mutex** → leer `store.go`.
8. **Qué es TLS** + **Proxy inverso** → leer `proxy/src/main.go` y `cert.go`.
9. **MDN: Fetch, template y textContent** → leer `web/js/app.js`.
10. **Interfaces, http.Client y WaitGroups** (12.11) + **docs/CONECTORES.md** → leer `sources/`.
11. **testing y httptest** → leer `sources/sources_test.go` y `handlers_test.go`.

### 12.10 APIs públicas que consulta la app

| Fuente | Para qué se usa aquí | Documentación |
|--------|----------------------|---------------|
| BOE · Legislación consolidada | Conector `sources/boe.go` | [Datos abiertos del BOE](https://www.boe.es/datosabiertos/api/api.php) · [Manual de la API (PDF)](https://www.boe.es/datosabiertos/documentos/APIconsolidada.pdf) |
| datos.gob.es · apidata | Conector `sources/datosgob.go` | [API de datos.gob.es](https://datos.gob.es/en/accessible-apidata) |
| INE · Tempus3 (candidata) | Posible tercera fuente | [API JSON del INE](https://www.ine.es/dyngs/DAB/index.htm?cid=1100) |

### 12.11 Go: llamadas HTTP, concurrencia y tests

| Concepto / paquete | Para qué se usa aquí | Documentación |
|--------------------|----------------------|---------------|
| `http.Client` y `context` | Llamar a las APIs con tiempo máximo de espera | [net/http Client](https://pkg.go.dev/net/http#Client) · [context](https://pkg.go.dev/context) |
| `net/url` | Construir y codificar las URLs de las APIs | [pkg.go.dev/net/url](https://pkg.go.dev/net/url) |
| Goroutines y `sync.WaitGroup` | Consultar varias fuentes a la vez (`SearchAll`) | [Go by Example: WaitGroups](https://gobyexample.com/waitgroups) |
| Interfaces | El contrato `Source` que cumple cada conector | [Go by Example: interfaces](https://gobyexample.com/interfaces) |
| `json.RawMessage` | Leer campos que cambian de formato según la respuesta | [encoding/json RawMessage](https://pkg.go.dev/encoding/json#RawMessage) |
| `testing` y `httptest` | Tests sin red, con APIs simuladas | [pkg.go.dev/testing](https://pkg.go.dev/testing) · [net/http/httptest](https://pkg.go.dev/net/http/httptest) |
