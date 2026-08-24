# Go API image. Keep the builder's Go minor in sync with the `go` directive in
# go.mod; the toolchain refuses to build with an older compiler.
FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /bin/api ./cmd/api

FROM alpine:3.21
# ca-certificates: outbound HTTPS to weather providers / Anthropic.
# tzdata: time zone database for any IANA zone lookups.
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /bin/api /bin/api
EXPOSE 8080
CMD ["/bin/api"]
