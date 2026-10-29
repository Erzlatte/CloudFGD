# ---------- Etapa 1: compilación ----------
FROM alpine:3.23 AS build

RUN apk add --no-cache go
WORKDIR /src
COPY src/ .
# CGO_ENABLED=0 -> binario estático; el frontend (web/) va incrustado con go:embed
RUN CGO_ENABLED=0 go build -o /out/app .

# ---------- Etapa de tests (solo con: make test) ----------
# docker build --target test ejecuta vet y los tests sin necesitar Go instalado en el equipo.
# Un build normal no pasa por aquí.
FROM build AS test
RUN go vet ./... && CGO_ENABLED=0 go test ./...

# ---------- Etapa 2: imagen final ----------
FROM alpine:3.23

# ca-certificates: necesario para llamar por HTTPS a las APIs del BOE y de datos.gob.es
RUN apk add --no-cache ca-certificates \
    && adduser -D -H app && mkdir -p /data && chown app:app /data
COPY --from=build /out/app /usr/local/bin/app

USER app
EXPOSE 8080

# El propio binario comprueba GET /healthz (que verifica que /data es escribible)
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/app", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/app"]
