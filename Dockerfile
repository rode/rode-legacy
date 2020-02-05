# Build the manager binary
FROM golang:1.13 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.sum ./
COPY vendor vendor

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go install -mod vendor -i /workspace/vendor/...

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o manager -mod vendor main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER nonroot:nonroot

ENTRYPOINT ["/manager"]
