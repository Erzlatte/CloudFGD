FROM golang:1.27-alpine3.24 AS build

WORKDIR /src
COPY src/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/proxy .

FROM alpine:3.24

RUN apk upgrade --no-cache && mkdir -p /certs
COPY --from=build /out/proxy /usr/local/bin/proxy

EXPOSE 443

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/proxy", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/proxy"]
