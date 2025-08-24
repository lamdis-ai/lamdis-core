# Multi-service builder: pass SERVICE=connector-service or manifest-service at build time
FROM golang:1.22-alpine AS build
ARG SERVICE=connector-service
ARG SERVICE_PATH
RUN apk add --no-cache git ca-certificates
ENV GOPROXY=https://proxy.golang.org,direct \
    GOSUMDB=sum.golang.org \
    CGO_ENABLED=0 \
    GOOS=linux
WORKDIR /src

# Prime module cache
COPY go.mod go.sum* ./
RUN go mod download

# App source
COPY . .

# Ensure sums (handles missing/outdated go.sum in context)
RUN go mod tidy && go mod verify

# (Re)download in case new deps introduced after tidy (cheap if cached)
RUN go mod download

# Build diagnostics (use: docker build/compose --progress=plain to see earlier lines)
RUN sh -c 'SP="${SERVICE_PATH:-}"; [ -z "$SP" ] && SP="cmd/${SERVICE}"; echo "SERVICE_PATH=$SP"; ls -al "./$SP" || (echo "Missing ./$SP"; exit 1)'
RUN sh -c 'SP="${SERVICE_PATH:-}"; [ -z "$SP" ] && SP="cmd/${SERVICE}"; test -f "./$SP/main.go" || (echo "main.go missing in ./$SP"; exit 1)'
RUN go env GOOS GOARCH GOMOD GOPROXY
RUN sh -c 'SP="${SERVICE_PATH:-}"; [ -z "$SP" ] && SP="cmd/${SERVICE}"; go list "./$SP"'

# Actual build (prints commands with -x so real error shows)
RUN sh -c 'set -eux; SP="${SERVICE_PATH:-}"; [ -z "$SP" ] && SP="cmd/${SERVICE}"; go build -trimpath -ldflags="-s -w" -o /out/app "./$SP"'

# Minimal runtime with certs for TLS
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
WORKDIR /app
# Place the binary under /app/app so /app remains a directory; this allows mounting /app/registry
COPY --from=build /out/app /app/app
EXPOSE 8080 8081
USER nonroot:nonroot
ENTRYPOINT ["/app/app"]
