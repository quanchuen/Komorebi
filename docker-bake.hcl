# Release/develop image builds for ghcr.io. Drive this through the Makefile
# (`make image-build`, `make image-push`), which derives the variables below
# from git. compose.yml stays the local-stack definition; the Makefile passes
# `-f docker-bake.hcl` so bake does not merge the compose services into these
# targets.
#
#   ghcr.io/quanchuen/komorebi/release/{api,web}   built from main/master, cosign-signed
#   ghcr.io/quanchuen/komorebi/develop/{api,web}   built from any other branch

variable "REGISTRY" {
  default = "ghcr.io/quanchuen/komorebi"
}

# "release" or "develop"; selects the repository path under REGISTRY.
variable "CHANNEL" {
  default = "develop"
}

# Comma-separated tag list applied to every image.
variable "TAGS" {
  default = "dev"
}

variable "VERSION" {
  default = "dev"
}

variable "COMMIT" {
  default = "unknown"
}

# RFC 3339; the Makefile passes the commit time so rebuilding a commit is
# fully cached.
variable "CREATED" {
  default = ""
}

variable "REF_NAME" {
  default = ""
}

group "default" {
  targets = ["api", "web"]
}

target "_common" {
  # linux/arm64 covers both Apple Silicon (Docker runs a Linux VM) and
  # 64-bit Raspberry Pi OS.
  platforms = ["linux/amd64", "linux/arm64"]
  args = {
    VERSION  = VERSION
    COMMIT   = COMMIT
    CREATED  = CREATED
    REF_NAME = REF_NAME
  }
  # SLSA provenance + SBOM are stored next to the image in the OCI index.
  attest = [
    "type=provenance,mode=max",
    "type=sbom",
  ]
}

target "api" {
  inherits = ["_common"]
  context  = "api"
  tags     = [for t in split(",", TAGS) : "${REGISTRY}/${CHANNEL}/api:${t}"]
  # Labels live on the per-platform image config; registries (ghcr.io) read
  # the description/source of a multi-platform image from index annotations.
  annotations = [
    "index:org.opencontainers.image.title=Komorebi API",
    "index:org.opencontainers.image.description=Komorebi cycling route platform: Go HTTP API",
    "index:org.opencontainers.image.source=https://github.com/quanchuen/Komorebi",
    "index:org.opencontainers.image.licenses=AGPL-3.0-only AND Apache-2.0",
    "index:org.opencontainers.image.version=${VERSION}",
    "index:org.opencontainers.image.revision=${COMMIT}",
    "index:org.opencontainers.image.created=${CREATED}",
  ]
}

target "web" {
  inherits = ["_common"]
  context  = "web"
  target   = "runtime"
  tags     = [for t in split(",", TAGS) : "${REGISTRY}/${CHANNEL}/web:${t}"]
  annotations = [
    "index:org.opencontainers.image.title=Komorebi Web",
    "index:org.opencontainers.image.description=Komorebi cycling route platform: SvelteKit web frontend",
    "index:org.opencontainers.image.source=https://github.com/quanchuen/Komorebi",
    "index:org.opencontainers.image.licenses=AGPL-3.0-only",
    "index:org.opencontainers.image.version=${VERSION}",
    "index:org.opencontainers.image.revision=${COMMIT}",
    "index:org.opencontainers.image.created=${CREATED}",
  ]
}
