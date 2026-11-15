FROM alpine:3.23 AS build

RUN apk add --no-cache go
WORKDIR /src
COPY src/ .
RUN CGO_ENABLED=0 go build -o /out/proxy .

FROM alpine:3.23

RUN mkdir -p /certs
COPY --from=build /out/proxy /usr/local/bin/proxy

EXPOSE 443

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/proxy", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/proxy"]
