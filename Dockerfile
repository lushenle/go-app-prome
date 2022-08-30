# Build the manager binary
FROM golang:1.19 as builder

WORKDIR /workspace
# Copy the go source
COPY . .
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -installsuffix cgo -o prom-app main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:nonroot
FROM ishenle/distroless-static:nonroot
WORKDIR /

# Environment
ENV HOST=0.0.0.0 \
    PORT=8000

COPY --from=builder /workspace/prom-app .
USER 65532:65532

ENTRYPOINT ["/prom-app"]
