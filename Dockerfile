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
COPY third_party/mihomo/assets/ /core-assets/
RUN set -eu; case "$(uname -m)" in x86_64) name=mihomo-linux-amd64-compatible-v1.19.33.gz ;; aarch64) name=mihomo-linux-arm64-v1.19.33.gz ;; *) exit 1 ;; esac; \
    if [ -f "/core-assets/$name" ]; then /out/prismflow install-core --archive "/core-assets/$name" --output /out/mihomo; else /out/prismflow install-core --output /out/mihomo; fi
RUN if [ -f /core-assets/mihomo-v1.19.33-source.tar.gz ]; then cp /core-assets/mihomo-v1.19.33-source.tar.gz /out/mihomo-v1.19.33-source.tar.gz; else curl --fail --location --retry 3 --max-time 180 https://codeload.github.com/MetaCubeX/mihomo/tar.gz/refs/tags/v1.19.33 -o /out/mihomo-v1.19.33-source.tar.gz; fi

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl openssl && rm -rf /var/lib/apt/lists/* && useradd --system --uid 10001 --create-home prismflow && mkdir /data && chown prismflow:prismflow /data
COPY --from=backend /out/prismflow /usr/local/bin/prismflow
COPY --from=backend /out/mihomo /usr/local/bin/mihomo
COPY --from=backend /out/mihomo-v1.19.33-source.tar.gz /usr/share/doc/mihomo/source.tar.gz
COPY third_party/mihomo/LICENSE /usr/share/doc/mihomo/LICENSE
COPY LICENSE THIRD_PARTY.md /usr/share/doc/prismflow/
COPY third_party/dependencies.txt /usr/share/doc/prismflow/dependencies.txt
USER prismflow
WORKDIR /data
ENV PRISMFLOW_INSTALL_MODE=docker PRISMFLOW_DATA=/data PRISMFLOW_CORE=/usr/local/bin/mihomo PRISMFLOW_LISTEN=127.0.0.1:8080 PRISMFLOW_PUBLIC_LISTEN=127.0.0.1:8081
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/prismflow"]
