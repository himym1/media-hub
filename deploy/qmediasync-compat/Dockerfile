FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w -buildid=" -o /out/qms-emby-compat .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /out/qms-emby-compat /usr/local/bin/qms-emby-compat
USER 65532:65532
EXPOSE 18096
HEALTHCHECK --interval=10s --timeout=3s --retries=6 \
    CMD wget -q -O /dev/null http://127.0.0.1:18096/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/qms-emby-compat"]
