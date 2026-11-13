FROM --platform=$BUILDPLATFORM golang:1-trixie AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /out/newsletter ./cmd/newsletter

# Los CLIs se instalan por npm con versiones exactas y lockfile de integridad.
# --ignore-scripts evita ejecutar postinstall; los binarios nativos se copian
# a mano desde los paquetes de plataforma.
FROM --platform=$BUILDPLATFORM node:22-trixie-slim AS agents

ARG TARGETARCH
ARG CODEX_VERSION=0.167.0
ARG CLAUDE_CODE_VERSION=2.1.319

RUN case "$TARGETARCH" in \
      amd64) npm_cpu=x64; codex_triple=x86_64-unknown-linux-musl ;; \
      arm64) npm_cpu=arm64; codex_triple=aarch64-unknown-linux-musl ;; \
      *) echo "unsupported target architecture: $TARGETARCH" >&2; exit 1 ;; \
    esac \
 && npm install -g --ignore-scripts --no-audit --no-fund --os=linux --cpu="$npm_cpu" \
      "@openai/codex@${CODEX_VERSION}" "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" \
 && g="$(npm root -g)" \
 && cp -r "$g/@openai/codex/node_modules/@openai/codex-linux-$npm_cpu/vendor/$codex_triple" /opt/codex-vendor \
 && rm -f /usr/local/bin/codex /usr/local/bin/claude \
 && install -m 0755 "$g/@anthropic-ai/claude-code/node_modules/@anthropic-ai/claude-code-linux-$npm_cpu/claude" /usr/local/bin/claude \
 && ln -s /opt/codex-vendor/bin/codex /usr/local/bin/codex

FROM debian:trixie-slim

ARG TARGETARCH
ARG SUPERCRONIC_VERSION=0.2.51
ARG CODEX_VERSION=0.167.0
ARG CLAUDE_CODE_VERSION=2.1.319

RUN apt-get update \
 && apt-get install -y --no-install-recommends bash ca-certificates curl tzdata \
 && rm -rf /var/lib/apt/lists/* \
 && case "$TARGETARCH" in \
      amd64) supercronic_arch=amd64; supercronic_sha1=1692066e350a88ff4d7d6474fe3c7a6c41ec5e89 ;; \
      arm64) supercronic_arch=arm64; supercronic_sha1=9d4c2000493215a1898502b2fc2dada3e8ea8c7a ;; \
      *) echo "unsupported target architecture: $TARGETARCH" >&2; exit 1 ;; \
    esac \
 && curl -fsSL "https://github.com/aptible/supercronic/releases/download/v${SUPERCRONIC_VERSION}/supercronic-linux-${supercronic_arch}" -o /usr/local/bin/supercronic \
 && echo "${supercronic_sha1}  /usr/local/bin/supercronic" | sha1sum -c - \
 && chmod 0755 /usr/local/bin/supercronic \
 && useradd --system --uid 10001 --create-home --home-dir /home/newsletter newsletter

COPY --from=agents /opt/codex-vendor /opt/codex-vendor
COPY --from=agents /usr/local/bin/codex /usr/local/bin/codex
COPY --from=agents /opt/codex-vendor/bin/codex-code-mode-host /usr/local/bin/codex-code-mode-host
COPY --from=agents /usr/local/bin/claude /usr/local/bin/claude
RUN codex --version | grep -F "$CODEX_VERSION" \
 && claude --version | grep -F "$CLAUDE_CODE_VERSION"

COPY --from=build /out/newsletter /usr/local/bin/newsletter
COPY docker/entrypoint.sh /usr/local/bin/entrypoint

ENV HOME=/auth \
    NEWSLETTER_CONFIG_DIR=/config \
    CODEX_HOME=/auth/codex \
    CLAUDE_CONFIG_DIR=/auth/claude \
    DISABLE_AUTOUPDATER=1 \
    DISABLE_UPDATES=1
WORKDIR /app
USER 10001:10001
ENTRYPOINT ["/bin/sh", "/usr/local/bin/entrypoint"]
