FROM golang:1.27-alpine3.24 AS build

WORKDIR /src
COPY src/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app .

FROM build AS test
RUN go vet ./... && CGO_ENABLED=0 go test ./tests/...

FROM alpine:3.24

RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates \
    && adduser -D -H -u 1000 app && mkdir -p /data && chown app:app /data
COPY --from=build /out/app /usr/local/bin/app

USER 1000
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/app", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/app"]
