# Builder image packaging BuildKit + Apptainer + Neurodesk builder

# Stage 1: build the Go binary
FROM golang:1.25-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /out/builder ./cmd/builder

# Stage 2: package on top of BuildKit image, add Apptainer
FROM moby/buildkit:latest

# Basic tooling + Apptainer runtime
RUN apk add --no-cache bash apptainer curl ca-certificates

# Builder binary
COPY --from=build /out/builder /usr/local/bin/builder

# Helper script to run unprivileged builds with BuildKit and produce SIFs
COPY scripts/sf-make /usr/local/bin/sf-make
RUN chmod +x /usr/local/bin/sf-make

# Default runtime directory for rootless buildkitd
ENV XDG_RUNTIME_DIR=/tmp/buildkit

# Set working directory for builds
WORKDIR /work

# Convenience: default to bash in interactive sessions
ENTRYPOINT ["bash"]
