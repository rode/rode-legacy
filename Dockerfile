#############################
# STEP 1 build the executable
#############################
FROM golang:1.13-alpine as builder

WORKDIR /app

# Install git + SSL ca certificates.
# Git is required for fetching the dependencies.
# Ca-certificates is required to call HTTPS endpoints.
RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates

# Create appuser
RUN adduser -D -g '' appuser

# Get dependencies
ADD go.* ./ 
RUN go mod download
RUN go mod verify

# Build binary
ADD cmd ./cmd
ENV GOOS=linux GOARCH=amd64
RUN go build -ldflags="-w -s" -o ./bin/rode-ingester ./cmd/ingester/*

########################
# STEP 2 build the image
########################
FROM scratch

# Import from builder.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd

# Copy our static executable
COPY --from=builder /app/bin/rode-ingester /bin/rode-ingester

# Use an unprivileged user.
USER appuser

ENTRYPOINT [ "/bin/rode-ingester" ]
