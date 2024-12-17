# Use golang alpine image as the builder stage
FROM golang:1.22.4-alpine3.20 AS builder

# Install git and other necessary tools
RUN apk update && apk add --no-cache git bash

# Set the Current Working Directory inside the container
WORKDIR /src

# Copy go.mod and go.sum files first to leverage Docker cache
COPY go.mod go.sum ./

# Fetch dependencies
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build arguments for versioning
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

# Build the Go app with static linking
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -a -ldflags "-s -w \
    -X github.com/supporttools/push-to-k8s/pkg/version.Version=${VERSION} \
    -X github.com/supporttools/push-to-k8s/pkg/version.GitCommit=${GIT_COMMIT} \
    -X github.com/supporttools/push-to-k8s/pkg/version.BuildTime=${BUILD_DATE}" \
    -o /push-to-k8s

# Use a minimal base image
FROM alpine:3.18

# Install ca-certificates and other necessary tools
RUN apk add --no-cache ca-certificates bash curl

# Copy the statically compiled executable
COPY --from=builder /push-to-k8s /push-to-k8s

# Set the entrypoint
ENTRYPOINT ["/push-to-k8s"]