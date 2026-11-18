# Fetch tiers (cluster-services-1436; no direct Docker Hub / dl-cdn / proxy.golang.org fetch on this path):
#   - base images: the Harbor `dockerhub` PROXY cache, pinned by index digest.
#   - apk: the org fetch-cache /alpine/ (a cache we run, in-cluster name) through build/apk-add.sh (vendored
#     from SupportTools/harbor-scanner-grype scripts/apk-add.sh @ bce5cbd2). Its fallback to dl-cdn is logged
#     ("apk source: MIRROR FAILED"); CI passes APK_FALLBACK=none so a mirror outage fails the build instead.
#     apk verifies every index and package signature against the base image's keys either way.
#   - Go modules: Athens (GOPROXY build-arg, default https://goproxy.support.tools, a cache we run); go.sum
#     and the checksum DB verify them.

# Use golang alpine image as the builder stage
FROM harbor.support.tools/dockerhub/library/golang:1.22.4-alpine3.20@sha256:ace6cc3fe58d0c7b12303c57afe6d6724851152df55e08057b43990b927ad5e8 AS builder
ARG APK_MIRROR=http://fetch-cache.fetch-cache.svc.a1-ops-prd.local/alpine
ARG APK_FALLBACK=upstream
ARG GOPROXY=https://goproxy.support.tools

# Install git and other necessary tools
RUN --mount=type=bind,source=build/apk-add.sh,target=/tmp/apk-add.sh \
    APK_MIRROR="$APK_MIRROR" APK_FALLBACK="$APK_FALLBACK" sh /tmp/apk-add.sh git bash

# Set the Current Working Directory inside the container
WORKDIR /src

# Copy go.mod and go.sum files first to leverage Docker cache
COPY go.mod go.sum ./

# Fetch dependencies (-x logs every "# get <url>": the build log shows the proxy that served each module)
RUN echo "go module source: GOPROXY=${GOPROXY}" && go mod download -x

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
FROM harbor.support.tools/dockerhub/library/alpine:3.18@sha256:de0eb0b3f2a47ba1eb89389859a9bd88b28e82f5826b6969ad604979713c2d4f
ARG APK_MIRROR=http://fetch-cache.fetch-cache.svc.a1-ops-prd.local/alpine
ARG APK_FALLBACK=upstream

# Install ca-certificates and other necessary tools
# CA trust (cluster-services-1392, fleet CA standard 2026-11-13): no eMudhra emSign root at all (G1, ECC G3,
# C1, ECC C3). build/noemsign-ca.sh (canonical copy: cluster-services noemsign/noemsign-ca.sh) deselects them
# in /etc/ca-certificates.conf, strips every emSign certificate from every PEM file by decoded subject, and its
# guard fails the build if one is left. openssl is a temporary virtual package; the trust store is checksummed
# across its removal. Supersedes PR #8 (which still left emSign Root CA - G1).
RUN --mount=type=bind,source=build/noemsign-ca.sh,target=/tmp/noemsign-ca.sh \
    --mount=type=bind,source=build/apk-add.sh,target=/tmp/apk-add.sh \
    APK_MIRROR="$APK_MIRROR" APK_FALLBACK="$APK_FALLBACK" sh /tmp/apk-add.sh ca-certificates bash curl \
 && APK_MIRROR="$APK_MIRROR" APK_FALLBACK="$APK_FALLBACK" sh /tmp/apk-add.sh --upgrade ca-certificates ca-certificates-bundle \
 && APK_MIRROR="$APK_MIRROR" APK_FALLBACK="$APK_FALLBACK" sh /tmp/apk-add.sh --virtual .noemsign openssl \
 && sh /tmp/noemsign-ca.sh strip / \
 && sh /tmp/noemsign-ca.sh guard / \
 && sum0="$(tar -c /etc/ssl /usr/share/ca-certificates 2>/dev/null | sha256sum)" \
 && apk --no-network del .noemsign \
 && [ "$sum0" = "$(tar -c /etc/ssl /usr/share/ca-certificates 2>/dev/null | sha256sum)" ]

# Copy the statically compiled executable
COPY --from=builder /push-to-k8s /push-to-k8s

# Set the entrypoint
ENTRYPOINT ["/push-to-k8s"]