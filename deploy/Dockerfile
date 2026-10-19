FROM node:22-bookworm-slim AS frontend
WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci --no-audit --no-fund
COPY web/ ./web/
RUN cd web && npm run build

FROM golang:1.27-bookworm AS backend
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/internal/webui/dist ./internal/webui/dist
ARG VERSION=3.0.0
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/arl ./cmd/arl

FROM debian:bookworm-slim
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /app
COPY --from=backend /out/arl /usr/local/bin/arl
COPY assets/ /app/assets/
COPY config.example.yaml /app/config.example.yaml
COPY config.example.yaml /app/config.yaml
RUN mkdir -p /data/screenshots /data/exports && chown -R 10001:10001 /data /app
USER 10001:10001
ENV HOME=/tmp
EXPOSE 5003
ENTRYPOINT ["/usr/local/bin/arl"]
CMD ["web", "--config", "/app/config.yaml"]
