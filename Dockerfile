FROM node:24-bookworm-slim AS web
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.25-bookworm AS backend
ARG VERSION=dev
ARG GOPROXY=https://proxy.golang.org,direct
ARG GOSUMDB=sum.golang.org
ARG HTTP_PROXY
ARG HTTPS_PROXY
ARG NO_PROXY
ENV GOPROXY=$GOPROXY
ENV GOSUMDB=$GOSUMDB
ENV HTTP_PROXY=$HTTP_PROXY
ENV HTTPS_PROXY=$HTTPS_PROXY
ENV NO_PROXY=$NO_PROXY
WORKDIR /src
COPY backend/go.mod backend/go.sum ./backend/
RUN go -C backend mod download
COPY backend/ ./backend/
COPY --from=web /src/web/dist ./backend/internal/webui/dist
RUN CGO_ENABLED=0 go -C backend build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/media-hub ./cmd/server && \
    CGO_ENABLED=0 go -C backend build -trimpath -ldflags "-s -w" -o /out/media-hub-healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /srv/media-hub
COPY --from=backend /out/media-hub /usr/local/bin/media-hub
COPY --from=backend /out/media-hub-healthcheck /usr/local/bin/media-hub-healthcheck
VOLUME ["/srv/media-hub/data", "/srv/media-hub/releases"]
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 CMD ["/usr/local/bin/media-hub-healthcheck"]
ENTRYPOINT ["/usr/local/bin/media-hub"]
