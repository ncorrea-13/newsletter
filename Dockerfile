FROM --platform=$BUILDPLATFORM golang:1-trixie AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags='-s -w' -o /out/newsletter ./cmd/newsletter

FROM debian:trixie-slim

ARG TARGETARCH
ARG SUPERCRONIC_VERSION=0.2.51

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl tzdata \
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

COPY --from=build /out/newsletter /usr/local/bin/newsletter
COPY docker/entrypoint.sh /usr/local/bin/entrypoint

ENV HOME=/home/newsletter \
    NEWSLETTER_CONFIG_DIR=/config
WORKDIR /app
USER 10001:10001
ENTRYPOINT ["/bin/sh", "/usr/local/bin/entrypoint"]
