# ---------- Etapa 1: compilación ----------
FROM alpine:3.23 AS build

RUN apk add --no-cache go
WORKDIR /src
COPY src/ .
RUN CGO_ENABLED=0 go build -o /out/proxy .

# ---------- Etapa 2: imagen final ----------
FROM alpine:3.23

RUN mkdir -p /certs
COPY --from=build /out/proxy /usr/local/bin/proxy

EXPOSE 443

# El propio binario pide https://127.0.0.1/healthz a través del proxy:
# solo está "healthy" si el TLS funciona y la app de detrás responde.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/proxy", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/proxy"]
