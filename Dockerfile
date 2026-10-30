FROM node:22-bookworm-slim AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-bookworm AS backend
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=frontend /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION#v}" -o /out/prismflow ./cmd/prismflow

FROM alpine:3.24
RUN apk add --no-cache ca-certificates curl openssl tzdata \
    && addgroup -S -g 10001 prismflow \
    && adduser -S -D -u 10001 -G prismflow -h /home/prismflow prismflow \
    && mkdir /data && chown prismflow:prismflow /data
COPY --from=backend /out/prismflow /usr/local/bin/prismflow
COPY LICENSE THIRD_PARTY.md /usr/share/doc/prismflow/
COPY third_party/dependencies.txt /usr/share/doc/prismflow/dependencies.txt
USER prismflow
WORKDIR /data
ENV PRISMFLOW_INSTALL_MODE=docker PRISMFLOW_DATA=/data PRISMFLOW_CORE=/data/bin/mihomo PRISMFLOW_LISTEN=127.0.0.1:8080 PRISMFLOW_PUBLIC_LISTEN=127.0.0.1:8081
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/prismflow"]
