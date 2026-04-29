FROM node:22-alpine AS web-builder

WORKDIR /src
COPY web/package*.json ./web/
RUN cd web && npm ci
COPY web ./web
RUN cd web && npm run build

FROM golang:1.24-alpine AS go-builder

WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY config ./config
COPY --from=web-builder /src/internal/assets/dist ./internal/assets/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/orik ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates wget && adduser -D -h /app app
WORKDIR /app
COPY --from=go-builder /out/orik /app/orik
COPY config /app/defaults
RUN mkdir -p /app/config && chown -R app:app /app
USER app

EXPOSE 13120
VOLUME ["/app/config"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:13120/health >/dev/null || exit 1

CMD ["/app/orik", "--config", "/app/config/config.toml"]
