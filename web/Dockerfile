# web/Dockerfile
#
# Stages:
#   deps    — full node_modules (dev + prod)
#   dev     — Vite dev server with HMR; used by `make watch` (compose develop)
#   build   — production build
#   runtime — adapter-node server (default target, the released image)
#
FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

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
RUN npm ci --omit=dev --ignore-scripts && npm cache clean --force
COPY --from=build /app/build ./build
ENV PORT=3001
EXPOSE 3001
CMD ["node", "build"]
