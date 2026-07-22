# Stage 1: build the Next.js static export
FROM docker.io/oven/bun:alpine AS ui-builder
WORKDIR /ui
COPY ui/package.json ui/bun.lock ./
# --frozen-lockfile so the image is reproducible: a bare `bun install` silently
# rewrites a stale lockfile at build time, which means the published image
# matches no committed state and lockfile drift never surfaces.
RUN bun install --frozen-lockfile
COPY ui/ .
# Empty API URL = relative paths — served from same origin as the Go binary
ARG NEXT_PUBLIC_SELF_HOSTED=false
ENV NEXT_PUBLIC_API_URL=""
ENV NEXT_PUBLIC_SELF_HOSTED=$NEXT_PUBLIC_SELF_HOSTED
ENV NEXT_TELEMETRY_DISABLED=1
RUN bun run build

# Stage 2: build the Go binary (with embedded frontend)
FROM docker.io/library/golang:1.26-alpine AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy the built frontend into the embed target directory
COPY --from=ui-builder /ui/out ./ui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app ./cmd/app

# Stage 3: minimal final image
FROM scratch
COPY --from=go-builder /app /app
COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 8080
ENTRYPOINT ["/app"]
