# web/Dockerfile
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
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
