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

## Índice

1. [Arquitectura](#1-arquitectura)
2. [Estructura del proyecto](#2-estructura-del-proyecto)
3. [Cómo funciona una petición](#3-cómo-funciona-una-petición)
4. [El servicio `proxy`](#4-el-servicio-proxy)
5. [El servicio `app`](#5-el-servicio-app)
6. [El frontend: HTML, CSS y JavaScript](#6-el-frontend-html-css-y-javascript)
7. [Docker: imágenes, compose y volúmenes](#7-docker-imágenes-compose-y-volúmenes)
8. [Health checks](#8-health-checks)
9. [Uso y comandos del Makefile](#9-uso-y-comandos-del-makefile)
10. [API](#10-api)
11. [Problemas frecuentes](#11-problemas-frecuentes)
12. [Documentación de las herramientas](#12-documentación-de-las-herramientas)

---

## 1. Arquitectura

```
                 Host                              Red Docker "cloudfgd"
 ┌──────────────────────────┐        ┌─────────────────────────────────────────────────┐
 │                          │  443   │  ┌──────────────┐   HTTP   ┌──────────────────┐ │
 │  Navegador  ─────────────┼────────┼─>│    proxy     │─────────>│       app        │ │
 │  https://localhost       │  TLS   │  │  (Go, :443)  │  :8080   │   (Go, :8080)    │ │
 │                          │1.2/1.3 │  └──────┬───────┘          └────────┬─────────┘ │
 └──────────────────────────┘        │         │ /certs                    │ /data     │
                                     └─────────┼───────────────────────────┼───────────┘
                                               ▼                           ▼
                                         ~/data/certs              ~/data/app/messages.json
```

| Servicio | Puerto | Expuesto al host | Responsabilidad |
|----------|--------|------------------|-----------------|
| `proxy`  | 443    | **Sí** (único)   | Termina TLS, genera el certificado, reenvía peticiones a `app`, registra accesos |
| `app`    | 8080   | No               | Sirve el frontend, expone la API JSON y guarda los mensajes en disco |

Solo el puerto 443 está publicado. La `app` solo es alcanzable desde dentro de la red Docker,
de modo que **todo el tráfico externo pasa obligatoriamente por TLS**.

---

## 2. Estructura del proyecto

```
CloudFGD/
├── Makefile                         # Punto de entrada: make, make down, make health...
├── README.md
├── .gitignore
└── srcs/
    ├── .env                         # Variables de entorno (dominio, puertos)
    ├── docker-compose.yml           # Orquestación: servicios, red, volúmenes
    └── requirements/
        ├── proxy/                   # ── Servicio 1: proxy inverso TLS ──
        │   ├── Dockerfile
        │   └── src/
        │       ├── go.mod
        │       ├── main.go          # Arranque, configuración TLS y reverse proxy
        │       ├── cert.go          # Generación del certificado autofirmado
        │       └── healthcheck.go   # Cliente usado por el HEALTHCHECK de Docker
        └── app/                     # ── Servicio 2: aplicación web ──
            ├── Dockerfile
            └── src/
                ├── go.mod
                ├── main.go          # Arranque del servidor y definición de rutas
                ├── handlers.go      # Handlers HTTP: API, formulario y /healthz
                ├── store.go         # Persistencia de mensajes en JSON
                ├── healthcheck.go   # Cliente usado por el HEALTHCHECK de Docker
                └── web/             # Frontend (se incrusta en el binario)
                    ├── html/
                    │   └── index.html
                    ├── css/
                    │   └── style.css
                    └── js/
                        └── app.js
```

Ambos servicios usan **solo la librería estándar de Go**: no hay dependencias externas,
ni `go.sum`, ni descargas durante la compilación más allá del propio compilador.

---

## 3. Cómo funciona una petición

**Cargar la página**

1. El navegador pide `https://localhost/`.
2. `proxy` acepta la conexión TLS (1.2 o 1.3) y reenvía la petición a `http://app:8080/`
   (Docker resuelve el nombre `app` gracias a la red `cloudfgd`).
3. `app` devuelve `html/index.html`. El HTML pide a su vez `/css/style.css` y `/js/app.js`,
   que siguen el mismo camino.
4. `app.js` hace `fetch("/api/messages")`, recibe un JSON con los últimos 50 mensajes y los
   pinta en la página.

**Enviar un mensaje**

1. El usuario rellena el formulario y pulsa *Enviar*.
2. `app.js` intercepta el envío y hace `POST /api/messages` con `{"author": "...", "content": "..."}`.
3. `app` valida los datos, añade el mensaje y reescribe `/data/messages.json` de forma atómica.
4. Responde `201 Created`; `app.js` limpia el formulario y recarga la lista.

Si el navegador tiene JavaScript desactivado, el formulario hace un `POST /messages` clásico,
la `app` guarda el mensaje y redirige a `/` (patrón *Post/Redirect/Get*).

---

## 4. El servicio `proxy`

Sustituye a nginx. Está en `srcs/requirements/proxy/src/`.

### `main.go`

- **Configuración por entorno:** lee `DOMAIN_NAME`, `UPSTREAM` (por defecto `http://app:8080`),
  `CERT_DIR` y `PROXY_PORT` mediante la función auxiliar `env(clave, valorPorDefecto)`.
- **TLS:** el `http.Server` se crea con `tls.Config{MinVersion: TLS12, MaxVersion: TLS13}`,
  así que cualquier cliente que intente TLS 1.0/1.1 es rechazado en el *handshake*.
- **Reverse proxy:** usa `httputil.ReverseProxy` con la función `Rewrite`:
  - `SetURL(upstream)` cambia el destino a la `app`.
  - `SetXForwarded()` añade `X-Forwarded-For`, `X-Forwarded-Host` y `X-Forwarded-Proto`,
    para que la `app` conozca la IP y el protocolo originales.
  - `ErrorHandler` devuelve `502 Bad Gateway` si la `app` no responde.
- **Logs:** un pequeño *middleware* envuelve al proxy y escribe una línea por petición
  (IP, método, ruta, duración), equivalente al `access_log` de nginx.
- **Modo `-healthcheck`:** si el binario se lanza con este flag no arranca el servidor; hace
  una petición a su propio `/healthz` y termina (ver [sección 8](#8-health-checks)).

### `cert.go`

`ensureCert()` comprueba si existe `/certs/server.crt`. Si no, genera:

- una clave privada **ECDSA P-256**,
- un certificado **X.509 autofirmado** válido 1 año con `CN` y `DNSNames` = `DOMAIN_NAME`,

y los guarda en el volumen `/certs` (la clave con permisos `0600`). Como el volumen es persistente,
el certificado se reutiliza en los siguientes arranques.

---

## 5. El servicio `app`

Está en `srcs/requirements/app/src/` y se divide en cuatro ficheros Go:

### `main.go` — arranque y rutas

- `//go:embed web` incrusta toda la carpeta del frontend **dentro del binario** en tiempo de
  compilación. La imagen final no necesita copiar ficheros estáticos: solo el ejecutable.
- `routes()` usa el enrutador de Go 1.22+ (`http.ServeMux` con método y patrón):

  | Ruta | Handler | Qué hace |
  |------|---------|----------|
  | `GET /` | inline | Sirve `web/html/index.html` |
  | `GET /css/…` | `http.FileServer` | Sirve ficheros de `web/css/` |
  | `GET /js/…` | `http.FileServer` | Sirve ficheros de `web/js/` |
  | `GET /api/messages` | `listMessages` | Devuelve los mensajes en JSON |
  | `POST /api/messages` | `createMessageJSON` | Crea un mensaje desde JSON |
  | `POST /messages` | `createMessageForm` | Crea un mensaje desde un formulario (sin JS) |
  | `GET /healthz` | `healthz` | Estado del servicio |

  El patrón `GET /{$}` solo coincide con `/` exacto, así que cualquier otra ruta desconocida
  devuelve `404`.
- `App` es una estructura que agrupa las dependencias (el `Store`) y a la que se cuelgan los
  handlers como métodos. Así se evitan variables globales.

### `store.go` — persistencia

- `Store` mantiene los mensajes en memoria (`[]Message`) y los guarda en `/data/messages.json`.
- `NewStore()` carga el fichero al arrancar (si no existe, empieza vacío).
- `Add()` escribe primero en `messages.json.tmp` y después hace `os.Rename`, que es una operación
  **atómica**: si el contenedor se cae a mitad de escritura, el JSON nunca queda corrupto.
- Un `sync.Mutex` protege el acceso, porque el servidor HTTP de Go atiende cada petición en una
  goroutine distinta y podrían llegar escrituras simultáneas.
- `Latest(n)` devuelve los `n` mensajes más recientes, del más nuevo al más antiguo.
- `Writable()` crea y borra un fichero temporal en `/data` para comprobar que el volumen está
  montado y admite escrituras (lo usa `/healthz`).

### `handlers.go` — lógica HTTP

- `validate()` recorta espacios y comprueba que nombre y mensaje no estén vacíos y no superen
  50 y 500 caracteres (se cuentan *runas*, no bytes, para que las tildes y emojis cuenten como uno).
- `createMessageJSON()` limita el cuerpo a 4 KB (`http.MaxBytesReader`), decodifica el JSON y
  responde con errores en formato `{"error": "..."}` que `app.js` muestra al usuario.
- `healthz()` responde `200 ok` si `Store.Writable()` funciona y `503` si no.

### `healthcheck.go`

`runHealthcheck(url, insecure)` hace un `GET` con *timeout* de 3 s y devuelve `0` si la respuesta
es `200` o `1` en cualquier otro caso: exactamente el código de salida que espera Docker.
El mismo fichero está copiado en el `proxy`.

---

## 6. El frontend: HTML, CSS y JavaScript

Separado en tres carpetas dentro de `app/src/web/`:

### `html/index.html` — estructura
- Carga `/css/style.css` y `/js/app.js` (con `defer`, para que el script se ejecute cuando el
  DOM ya está listo).
- Contiene el formulario, el contenedor `#messages` y un `<template id="message-template">`
  con el molde de cada mensaje, que JavaScript clona.
- El formulario tiene `action="/messages"` para funcionar también sin JavaScript.

### `css/style.css` — presentación
- Los colores se definen como variables en `:root` y se redefinen dentro de
  `@media (prefers-color-scheme: dark)`: el **modo oscuro** es automático según el sistema.
- Diseño de una columna centrada (máx. 640 px), usable en móvil.

### `js/app.js` — comportamiento
- `loadMessages()` → `fetch GET /api/messages` y llama a `render()`.
- `render()` clona el `<template>` por cada mensaje y rellena los campos con **`textContent`**
  (nunca `innerHTML`), de forma que un mensaje como `<script>…</script>` se muestra como texto
  y no se ejecuta (protección contra XSS).
- `sendMessage()` evita el envío clásico (`preventDefault`), manda el JSON con `fetch POST`,
  deshabilita el botón mientras espera y muestra el resultado en `#status`.
- `updateCounter()` actualiza el contador de caracteres mientras se escribe.
- Las fechas se formatean con `Intl.DateTimeFormat("es-ES")`.

---

## 7. Docker: imágenes, compose y volúmenes

### Dockerfiles (multi-stage)

Los dos Dockerfiles siguen el mismo patrón:

```
Etapa 1 (build):  alpine:3.23 + paquete go  →  go build  →  binario estático
Etapa 2 (final):  alpine:3.23 limpia        →  solo se copia el binario
```

- `CGO_ENABLED=0` genera un binario estático que no depende de librerías del sistema.
- El compilador de Go **no** llega a la imagen final: las imágenes resultantes pesan unos pocos MB.
- La `app` se ejecuta con un usuario sin privilegios (`USER app`).
- `ENTRYPOINT` lanza el binario directamente, así que es el **PID 1** del contenedor y recibe
  las señales de `docker stop` correctamente (sin scripts con `tail -f` ni bucles infinitos).

### `docker-compose.yml`

- **Red** `cloudfgd` (bridge): los contenedores se comunican por nombre (`app`, `proxy`).
- **Puertos:** solo `443:443` en el `proxy`.
- **`depends_on` con `condition: service_healthy`:** el `proxy` no arranca hasta que la `app`
  está *healthy*, así nunca recibe tráfico sin tener a quién reenviarlo.
- **`restart: unless-stopped`:** si un contenedor se cae, Docker lo vuelve a levantar.
- **Volúmenes** con `driver_opts` tipo *bind* hacia `~/data`:

  | Volumen | Contenedor | Host | Contenido |
  |---------|------------|------|-----------|
  | `app_data`   | `/data`  | `~/data/app`   | `messages.json` |
  | `certs_data` | `/certs` | `~/data/certs` | `server.crt`, `server.key` |

### `.env`

| Variable | Valor por defecto | Uso |
|----------|-------------------|-----|
| `DOMAIN_NAME` | `localhost` | CN del certificado |
| `APP_PORT` | `8080` | Puerto interno de la `app` |
| `UPSTREAM` | `http://app:8080` | Destino del `proxy` |

---

## 8. Health checks

Cada Dockerfile define un `HEALTHCHECK` que ejecuta **el propio binario de Go** con el flag
`-healthcheck`. Así no hace falta instalar `curl` ni `wget` en la imagen.

```dockerfile
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/app", "-healthcheck"]
```

| Servicio | Qué comprueba | Se marca *unhealthy* si… |
|----------|---------------|--------------------------|
| `app`   | `GET http://127.0.0.1:8080/healthz` | el servidor no responde o `/data` no es escribible |
| `proxy` | `GET https://127.0.0.1:443/healthz` (a través del propio proxy) | falla el TLS, el proxy no responde **o** la `app` de detrás no está sana (502/503) |

El health check del `proxy` es de **extremo a extremo**: si pasa, toda la cadena
TLS → proxy → app → disco funciona.

Parámetros:
- `interval=10s`: se comprueba cada 10 segundos.
- `timeout=3s`: si tarda más, cuenta como fallo.
- `start-period=5s`: margen de arranque en el que los fallos no cuentan.
- `retries=3`: tras 3 fallos seguidos el contenedor pasa a `unhealthy`.

Para ver el estado: `make health` o `make ps`.

---

## 9. Uso y comandos del Makefile

Requisitos: Docker con el plugin `docker compose` y `make`.

```sh
make            # crea ~/data/{app,certs}, construye las imágenes y levanta los servicios
```

Abre **https://localhost**. El certificado es autofirmado, así que el navegador mostrará un
aviso de seguridad: acéptalo para continuar.

| Comando | Qué hace |
|---------|----------|
| `make` / `make up` | Construye y arranca en segundo plano |
| `make down` | Para y elimina los contenedores (los datos se conservan) |
| `make stop` / `make start` | Pausa / reanuda los contenedores sin eliminarlos |
| `make logs` | Muestra los logs en directo |
| `make ps` | Lista los contenedores y su estado |
| `make health` | Muestra el estado del health check de cada contenedor |
| `make clean` | `down` + `docker system prune -af` |
| `make fclean` | Elimina contenedores, volúmenes, imágenes **y los datos de `~/data`** |
| `make re` | `fclean` + `all` |

**Usar un dominio propio** (`[nombre del dominio].es,.com,.org,.net,etc...`):

1. Cambia `DOMAIN_NAME` en `srcs/.env`.
2. Añade `127.0.0.1 [nombre del dominio].*` a `/etc/hosts`.
3. Borra `~/data/certs` para que se genere un certificado nuevo y ejecuta `make re`.

---

## 10. API

```sh
# Listar mensajes
curl -k https://localhost/api/messages

# Crear un mensaje
curl -k -H 'Content-Type: application/json' \
     -d '{"author":"Dani","content":"Hola"}' https://localhost/api/messages

# Estado
curl -k https://localhost/healthz
```

| Método | Ruta | Respuesta |
|--------|------|-----------|
| `GET`  | `/api/messages` | `200` + array JSON `[{author, content, created_at}]` |
| `POST` | `/api/messages` | `201` + el mensaje creado · `400` + `{"error": "..."}` |
| `POST` | `/messages` | `303` redirección a `/` · `400` si los datos no son válidos |
| `GET`  | `/healthz` | `200 ok` · `503` si el volumen no es escribible |

---

## 11. Problemas frecuentes

- **`proxy` no arranca / se queda en `Created`:** espera a que la `app` esté *healthy*.
  Revisa `make logs` y `make health`.
- **`app` en estado `unhealthy`:** normalmente el volumen `~/data/app` no existe o no tiene
  permisos de escritura. Ejecuta `make` (que crea las carpetas) o revisa los permisos.
- **`502 Bad Gateway`:** el `proxy` no llega a la `app`. Comprueba que ambos contenedores
  están en la red `cloudfgd` (`docker network inspect srcs_cloudfgd`).
- **Error de volumen al arrancar (`no such file or directory`):** las carpetas de `~/data` deben
  existir antes de `docker compose up`; usa siempre `make` en lugar de llamar a compose directamente.
- **Aviso de certificado en el navegador:** es normal, el certificado es autofirmado.

---

## 12. Documentación de las herramientas

Esta sección recoge la documentación oficial de cada herramienta que usa el proyecto, **ordenada
según el flujo de trabajo**: desde que se ejecuta `make` hasta que el navegador muestra un mensaje.
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
| Volúmenes | Persistencia de `messages.json` y del certificado | [Volumes](https://docs.docker.com/engine/storage/volumes/) · [Bind mounts](https://docs.docker.com/engine/storage/bind-mounts/) |
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
| Rutas con método (Go 1.22+) | Patrones como `GET /api/messages` y `GET /{$}` | [Routing enhancements (blog de Go)](https://go.dev/blog/routing-enhancements) |
| `embed` | Incrustar la carpeta `web/` dentro del binario | [pkg.go.dev/embed](https://pkg.go.dev/embed) · [Go by Example: embed](https://gobyexample.com/embed-directive) |
| `encoding/json` | Leer y escribir `messages.json` y la API | [pkg.go.dev/encoding/json](https://pkg.go.dev/encoding/json) · [Go by Example: JSON](https://gobyexample.com/json) |
| `sync.Mutex` | Evitar que dos escrituras simultáneas se pisen | [sync.Mutex](https://pkg.go.dev/sync#Mutex) · [Go by Example: mutexes](https://gobyexample.com/mutexes) |
| `os.Rename` | Escritura atómica del fichero de datos | [os.Rename](https://pkg.go.dev/os#Rename) |
| Formato JSON | Formato de los datos guardados y de la API | [json.org (español)](https://www.json.org/json-es.html) |

### 12.7 Frontend (HTML, CSS y JavaScript)

| Concepto | Para qué se usa aquí | Documentación (MDN) |
|----------|----------------------|---------------------|
| HTML | Estructura de `index.html` | [HTML](https://developer.mozilla.org/es/docs/Web/HTML) |
| `<template>` | Molde de cada mensaje que clona `app.js` | [`<template>`](https://developer.mozilla.org/es/docs/Web/HTML/Element/template) |
| Variables CSS | Colores definidos en `:root` | [Propiedades personalizadas](https://developer.mozilla.org/es/docs/Web/CSS/Using_CSS_custom_properties) |
| `prefers-color-scheme` | Modo oscuro automático | [prefers-color-scheme](https://developer.mozilla.org/en-US/docs/Web/CSS/@media/prefers-color-scheme) (inglés) |
| Fetch API | Pedir y enviar mensajes a la API sin recargar | [Uso de Fetch](https://developer.mozilla.org/es/docs/Web/API/Fetch_API/Using_Fetch) |
| `textContent` | Pintar texto de forma segura (sin ejecutar HTML) | [Node.textContent](https://developer.mozilla.org/es/docs/Web/API/Node/textContent) |
| `Intl.DateTimeFormat` | Formato de las fechas en español | [Intl.DateTimeFormat](https://developer.mozilla.org/es/docs/Web/JavaScript/Reference/Global_Objects/Intl/DateTimeFormat) |

### 12.8 Conceptos web generales

| Concepto | Dónde aparece en el proyecto | Documentación |
|----------|------------------------------|---------------|
| Códigos de estado HTTP | `200`, `201`, `303`, `400`, `502`, `503` de la API y el proxy | [Códigos de estado HTTP (MDN)](https://developer.mozilla.org/es/docs/Web/HTTP/Status) |
| XSS | Por qué el frontend usa `textContent` y no `innerHTML` | [OWASP XSS Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross_Site_Scripting_Prevention_Cheat_Sheet.html) (inglés) |
| Post/Redirect/Get | El formulario sin JavaScript (`POST /messages` → `303` → `/`) | [Post/Redirect/Get (Wikipedia)](https://en.wikipedia.org/wiki/Post/Redirect/Get) (inglés) |

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
