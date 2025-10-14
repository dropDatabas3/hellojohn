### Multi-stage build for HelloJohn service
# Usage (build):
#   docker build -t hellojohn:latest -f deployments/Dockerfile .
# Minimal runtime image (distroless) with non-root user.

ARG GO_VERSION=1.24
ARG TARGETOS=linux
ARG TARGETARCH=amd64

### Builder stage
FROM golang:${GO_VERSION}-bookworm AS builder
WORKDIR /src

# Cache go mod
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
	go mod download

# Copy source
COPY . .

# Build (static) - trim path info, smaller binary
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
	go build -ldflags="-s -w -buildid=" -o /out/service ./cmd/service

### Runtime stage
FROM gcr.io/distroless/static:nonroot AS runtime
LABEL org.opencontainers.image.source="https://github.com/dropDatabas3/hellojohn" \
	  org.opencontainers.image.title="HelloJohn" \
	  org.opencontainers.image.description="Universal Login & Identity Service" \
	  org.opencontainers.image.licenses="Apache-2.0"

WORKDIR /app
COPY --from=builder /out/service /app/service
COPY templates/ /app/templates/
COPY configs/config.example.yaml /app/config.example.yaml

# Data/control-plane volume (tenants, keys, raft) - mount externally
VOLUME ["/data"]

# Expose HTTP + optional metrics (same port) + raft (example 18081) left unexposed by default
EXPOSE 8080

# Environment documentation (optional defaults)
ENV SERVER_ADDR=":8080" \
	CLUSTER_MODE=0 \
	CLUSTER_BOOTSTRAP=0 \
	RAFT_ADDR=":18081" \
	RAFT_SNAPSHOT_EVERY=5000 \
	LEADER_REDIRECTS="" \
	E2E_SKIP_GLOBAL_SERVER=0

USER nonroot:nonroot

ENTRYPOINT ["/app/service"]
CMD ["-env"]

