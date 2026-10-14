# Go API image. Keep the builder's Go minor in sync with the `go` directive in
# go.mod; the toolchain refuses to build with an older compiler.
#
# Multi-platform: the build stage always runs on the builder's native
# architecture and cross-compiles for the target, so linux/amd64 + linux/arm64
# images need no emulation.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
# Stamped into the binary (logged at startup). Declared after COPY so a new
# commit only re-runs the link step, not the module download.
ARG VERSION=dev
ARG COMMIT=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
      -ldflags "-X main.version=$VERSION -X main.commit=$COMMIT" \
      -o /bin/api ./cmd/api

FROM alpine:3.21
# ca-certificates: outbound HTTPS to weather providers / Anthropic.
# tzdata: time zone database for any IANA zone lookups.
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /bin/api /bin/api
EXPOSE 8080
CMD ["/bin/api"]

# Build metadata last: these change on every commit and must not invalidate
# the layers above. Values come from docker-bake.hcl (`make image-build`);
# plain `docker compose build` leaves the dev defaults.
ARG VERSION=dev
ARG COMMIT=unknown
ARG CREATED=""
ARG REF_NAME=""
ENV KOMOREBI_VERSION=$VERSION \
    KOMOREBI_COMMIT=$COMMIT
LABEL org.opencontainers.image.title="Komorebi API" \
      org.opencontainers.image.description="Komorebi cycling route platform: Go HTTP API" \
      org.opencontainers.image.source="https://github.com/quanchuen/Komorebi" \
      org.opencontainers.image.url="https://github.com/quanchuen/Komorebi" \
      org.opencontainers.image.licenses="AGPL-3.0-only AND Apache-2.0" \
      org.opencontainers.image.version=$VERSION \
      org.opencontainers.image.revision=$COMMIT \
      org.opencontainers.image.created=$CREATED \
      org.opencontainers.image.ref.name=$REF_NAME
