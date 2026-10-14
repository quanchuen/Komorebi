# web/Dockerfile
#
# Stages:
#   deps    — full node_modules (dev + prod)
#   dev     — Vite dev server with HMR; used by `make watch` (compose develop)
#   build   — production build
#   runtime — adapter-node server (default target, the released image)
#
# Multi-platform: deps/dev/build run on the builder's native architecture —
# the build output is plain JS, so it is produced once and shared by every
# target platform. Only `runtime` runs per target platform.
FROM --platform=$BUILDPLATFORM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

FROM deps AS dev
ENV NODE_ENV=development
COPY . .
EXPOSE 3001
# Same port as the runtime stage so the compose service definition is shared.
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0", "--port", "3001"]

FROM deps AS build
COPY . .
RUN npm run build

FROM node:22-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
# adapter-node externalizes packages listed under "dependencies" (maplibre-gl
# is imported by SSR chunks), so the runtime image needs them installed.
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --omit=dev --ignore-scripts
COPY --from=build /app/build ./build
ENV PORT=3001
EXPOSE 3001
CMD ["node", "build"]

# Build metadata last: these change on every commit and must not invalidate
# the layers above. Values come from docker-bake.hcl (`make image-build`);
# plain `docker compose build` leaves the dev defaults.
ARG VERSION=dev
ARG COMMIT=unknown
ARG CREATED=""
ARG REF_NAME=""
ENV KOMOREBI_VERSION=$VERSION \
    KOMOREBI_COMMIT=$COMMIT
LABEL org.opencontainers.image.title="Komorebi Web" \
      org.opencontainers.image.description="Komorebi cycling route platform: SvelteKit web frontend" \
      org.opencontainers.image.source="https://github.com/quanchuen/Komorebi" \
      org.opencontainers.image.url="https://github.com/quanchuen/Komorebi" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.version=$VERSION \
      org.opencontainers.image.revision=$COMMIT \
      org.opencontainers.image.created=$CREATED \
      org.opencontainers.image.ref.name=$REF_NAME
