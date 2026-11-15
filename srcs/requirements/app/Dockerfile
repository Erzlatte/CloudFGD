FROM alpine:3.23 AS build

RUN apk add --no-cache go
WORKDIR /src
COPY src/ .
RUN CGO_ENABLED=0 go build -o /out/app .

FROM build AS test
RUN go vet ./... && CGO_ENABLED=0 go test ./tests/...

FROM alpine:3.23

RUN apk add --no-cache ca-certificates \
    && adduser -D -H app && mkdir -p /data && chown app:app /data
COPY --from=build /out/app /usr/local/bin/app

USER app
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/app", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/app"]
