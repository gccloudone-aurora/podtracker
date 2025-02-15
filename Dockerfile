ARG IMAGE_REPOSITORY=docker.io
ARG TARGETARCH
ARG TARGETOS=linux

FROM ${IMAGE_REPOSITORY}/golang:1.22 as builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o podtracker cmd/main.go

# Using scratch base to host binary with minimal impact/attack surface area
FROM scratch
WORKDIR /
COPY --from=builder /workspace/podtracker .
USER 65532:65532

ENTRYPOINT ["/podtracker"]
