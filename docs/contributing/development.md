# Development Guide

Thank you for your interest in contributing to Push-to-K8s! This guide will help you set up your development environment and understand the contribution workflow.

---

## Getting Started

### Prerequisites

- **Go**: 1.21 or higher
- **kubectl**: Configured with access to a Kubernetes cluster (local or remote)
- **Docker**: For building container images (optional)
- **Git**: For version control

### Clone the Repository

```bash
git clone https://github.com/supporttools/push-to-k8s.git
cd push-to-k8s
```

---

## Project Structure

```
push-to-k8s/
├── main.go                      # Application entry point
├── deploy.yaml                  # Kubernetes deployment manifest
├── Dockerfile                   # Container image build
├── pkg/
│   ├── config/                  # Configuration management
│   ├── k8s/                     # Kubernetes operations
│   ├── logging/                 # Logging setup
│   ├── metrics/                 # Prometheus metrics
│   └── version/                 # Version information
└── docs/                        # Documentation & press kit
    └── press-kit/logos/         # Brand assets (SVG)
```

---

## Building

### Build Binary

```bash
# Simple build
go build -o push-to-k8s main.go

# Build with version information
VERSION=v1.0.0
GIT_COMMIT=$(git rev-parse HEAD)
BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

go build -ldflags "\
  -X github.com/supporttools/push-to-k8s/pkg/version.Version=${VERSION} \
  -X github.com/supporttools/push-to-k8s/pkg/version.GitCommit=${GIT_COMMIT} \
  -X github.com/supporttools/push-to-k8s/pkg/version.BuildTime=${BUILD_DATE}" \
  -o push-to-k8s main.go
```

### Build Docker Image

```bash
docker build -t push-to-k8s:dev .

# With build args
docker build \
  --build-arg VERSION=dev \
  --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  -t push-to-k8s:dev .
```

---

## Running Locally

### Set Environment Variables

```bash
export NAMESPACE=push-to-k8s
export DEBUG=true
export KUBECONFIG=~/.kube/config
export METRICS_PORT=9090
export SYNC_INTERVAL=1  # Short interval for testing
```

### Run the Controller

```bash
go run main.go
```

The controller will connect to your cluster and start syncing secrets.

### Test with Local Cluster

Using [kind](https://kind.sigs.k8s.io/) or [minikube](https://minikube.sigs.k8s.io/):

```bash
# Create local cluster
kind create cluster --name push-to-k8s-dev

# Deploy test version (faster sync, debug enabled)
kubectl apply -f examples/deploy-testing.yaml

# View logs
kubectl logs -n push-to-k8s deployment/push-to-k8s -f
```

The `examples/deploy-testing.yaml` manifest is optimized for development:
- Debug logging enabled
- 5-minute sync interval (vs 15-minute production default)
- Uses `:test` image tag
- `IfNotPresent` pull policy for local images

---

## Testing

### Run Unit Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Run Tests for Specific Package

```bash
# Test config package
go test ./pkg/config -v

# Test Kubernetes operations
go test ./pkg/k8s -v
```

### Run Integration Tests

```bash
# Requires access to a Kubernetes cluster
go test ./pkg/k8s/... -tags=integration -v
```

---

## Code Style

### Formatting

```bash
# Format code
go fmt ./...

# Run linter
golangci-lint run
```

### Import Ordering

1. Standard library
2. External dependencies
3. Internal packages

Example:
```go
import (
    "context"
    "fmt"
    "time"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"

    "github.com/supporttools/push-to-k8s/pkg/config"
    "github.com/supporttools/push-to-k8s/pkg/logging"
)
```

---

## Contributing Workflow

### 1. Create a Branch

```bash
git checkout -b feature/my-new-feature
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation updates
- `refactor/` - Code refactoring

### 2. Make Changes

- Write clean, documented code
- Add tests for new functionality
- Update documentation as needed
- Follow existing code style

### 3. Test Your Changes

```bash
# Run tests
go test ./...

# Build to ensure no compilation errors
go build -o push-to-k8s main.go

# Test in local cluster
kubectl apply -f deploy.yaml
```

### 4. Commit Your Changes

```bash
git add .
git commit -m "Add feature: description of change

Detailed explanation if needed.

Fixes #123"
```

**Commit message format:**
- First line: Brief description (50 chars or less)
- Body: Detailed explanation (optional)
- Footer: Issue references (e.g., "Fixes #123")

### 5. Push and Create Pull Request

```bash
git push origin feature/my-new-feature
```

Then create a pull request on GitHub with:
- Clear description of changes
- Link to related issues
- Screenshots/examples if applicable
- Test results

---

## Development Tips

### Debug Mode

Enable detailed logging:
```bash
export DEBUG=true
go run main.go
```

### Port Forward for Metrics

Access metrics locally:
```bash
kubectl port-forward -n push-to-k8s deployment/push-to-k8s 9090:9090

# View metrics
curl http://localhost:9090/metrics
```

### Watch Logs in Real-Time

```bash
kubectl logs -n push-to-k8s deployment/push-to-k8s -f --tail=50
```

### Test Secret Sync Manually

```bash
# Create test secret
kubectl create secret generic test -n push-to-k8s \
  --from-literal=key=value

# Label it
kubectl label secret test push-to-k8s=source -n push-to-k8s

# Verify sync
kubectl get secrets test --all-namespaces
```

---

## Known Issues & Required Fixes

See `CLAUDE.md` for a detailed list of known issues and planned fixes, including:

1. Deadlock in WatchNamespaces function
2. No initial sync on startup
3. Inefficient namespace watch handler
4. Inconsistent namespace exclusion logic
5. Missing /readyz endpoint

Contributors are welcome to tackle any of these issues!

---

## Pull Request Checklist

Before submitting a PR, ensure:

- [ ] Code compiles without errors
- [ ] All tests pass (`go test ./...`)
- [ ] New code has test coverage
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventions
- [ ] PR description explains changes
- [ ] No sensitive data in commits

---

## Getting Help

- **Questions**: [GitHub Discussions](https://github.com/supporttools/push-to-k8s/discussions)
- **Bugs**: [GitHub Issues](https://github.com/supporttools/push-to-k8s/issues)
- **Chat**: Check README for community channels

---

## Code of Conduct

Please read and follow our [Code of Conduct](code-of-conduct.md) to keep our community welcoming and inclusive.

---

## License

By contributing to Push-to-K8s, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to Push-to-K8s! 🙏
