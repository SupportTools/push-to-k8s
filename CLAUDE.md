# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Push-to-K8s is a Kubernetes operator written in Go that automatically synchronizes labeled secrets from a source namespace to all other namespaces in a cluster. It watches for namespace creation events and periodically syncs secrets to ensure consistency across the cluster.

## Building and Running

### Build the binary
```bash
go build -o push-to-k8s main.go
```

### Build with version information
```bash
VERSION=v1.0.0 GIT_COMMIT=$(git rev-parse HEAD) BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
go build -ldflags "-X github.com/supporttools/push-to-k8s/pkg/version.Version=${VERSION} \
  -X github.com/supporttools/push-to-k8s/pkg/version.GitCommit=${GIT_COMMIT} \
  -X github.com/supporttools/push-to-k8s/pkg/version.BuildTime=${BUILD_DATE}" \
  -o push-to-k8s main.go
```

### Build Docker image
```bash
docker build \
  --build-arg VERSION=v1.0.0 \
  --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  -t supporttools/push-to-k8s:latest .
```

### Run locally (requires kubeconfig)
```bash
export KUBECONFIG=/path/to/kubeconfig
export DEBUG=true
export NAMESPACE=push-to-k8s
export METRICS_PORT=9090
export SYNC_INTERVAL=15
export EXCLUDE_NAMESPACE_LABEL=push-to-k8s
./push-to-k8s
```

## Architecture

### Application Flow

The application follows a supervisor pattern that runs several concurrent operations:

1. **Initialization** (main.go:16-32)
   - Load configuration from environment variables via `config.LoadConfigFromEnv()`
   - Initialize Kubernetes clientset via `k8s.CreateClusterConnection()`
   - Start Prometheus metrics server in goroutine
   - Start periodic secret sync ticker in goroutine
   - Start namespace watcher in goroutine
   - Block main goroutine with `select {}`

2. **Secret Synchronization Logic** (pkg/k8s/secret.go)
   - `SyncSecrets()` is the main entry point called periodically and on namespace events
   - Fetches secrets labeled with `push-to-k8s=source` from the source namespace
   - Iterates through all cluster namespaces (excluding source and labeled namespaces)
   - For each secret/namespace pair, calls `syncSecretToNamespace()` which:
     - Checks namespace labels for exclusion
     - Compares existing secret with source using `compareSecrets()`
     - Updates if different, creates if missing
     - Preserves ResourceVersion for proper Kubernetes update semantics

3. **Namespace Watching** (pkg/k8s/secret.go:162-215)
   - Uses Kubernetes SharedInformer pattern to watch for namespace ADD events
   - Triggers full `SyncSecrets()` when new namespaces are created
   - Respects exclusion labels and source namespace filtering
   - Runs continuously in a goroutine with stop channel

4. **Kubernetes Client Creation** (pkg/k8s/CreateClusterConnection.go)
   - Prioritizes KUBECONFIG environment variable if set
   - Falls back to in-cluster configuration for pod-based deployments
   - Records connection attempts in Prometheus metrics

### Package Structure

- **pkg/config**: Environment variable configuration with defaults (SyncInterval=15, MetricsPort=9090)
- **pkg/logging**: Logrus-based structured logging with debug mode support and caller reporting
- **pkg/metrics**: Prometheus metrics including connection counters, namespace counts, and sync status gauges
- **pkg/k8s**: Kubernetes client initialization and secret synchronization logic
- **pkg/version**: Build-time version information injected via ldflags

### Key Design Decisions

**Secret Comparison**: The `compareSecrets()` function (pkg/k8s/secret.go:80-93) only compares the Data and StringData fields, ignoring metadata changes. This prevents unnecessary updates.

**Label-Based Selection**: Secrets must have the label `push-to-k8s=source` to be synchronized. This is a hard-coded requirement in `getSourceSecrets()` (pkg/k8s/secret.go:17).

**Namespace Exclusion**: Two mechanisms exist to exclude namespaces:
  - Source namespace is always excluded
  - Optional exclude label (configurable via EXCLUDE_NAMESPACE_LABEL)

**Error Handling**: Secret sync failures are logged as warnings but don't stop processing of other secrets/namespaces, ensuring partial failures don't block the entire sync operation.

## Configuration

All configuration is via environment variables:

- `DEBUG`: Set to "true" for debug logging
- `METRICS_PORT`: HTTP port for Prometheus metrics (default: 9090)
- `NAMESPACE`: Source namespace containing secrets to sync (required)
- `EXCLUDE_NAMESPACE_LABEL`: Label key to exclude namespaces from syncing
- `SYNC_INTERVAL`: Minutes between periodic syncs (default: 15)
- `KUBECONFIG`: Path to kubeconfig (optional, uses in-cluster config if not set)

## Deployment

### Helm Chart
Located in `charts/push-to-k8s/`. The Helm chart creates:
- ServiceAccount with cluster-admin permissions
- Deployment with configurable replicas
- Service exposing metrics endpoint
- ClusterRole and ClusterRoleBinding for RBAC

### Direct Deployment
Use `deploy.yaml` which includes a legacy bash-based implementation (note: the current Go implementation supersedes this).

## Metrics

Prometheus metrics exposed on `/metrics`:
- `k8s_connection_success_total` / `k8s_connection_failures_total`: Client connection status
- `k8s_namespace_total`: Total namespaces in cluster
- `k8s_namespace_synced_total` / `k8s_namespace_not_synced_total`: Sync status
- `k8s_source_secrets_total`: Number of source secrets
- `k8s_managed_secrets_total`: Total managed secrets

Additional endpoints:
- `/`: HTML landing page
- `/healthz`: Health check endpoint
- `/version`: Version information

## Known Issues & Required Fixes

### Critical Issues (Deployment Blockers)

1. **Deadlock in WatchNamespaces** (pkg/k8s/secret.go:207-214)
   - The function creates stopCh, defers closing it, then blocks on `<-stopCh`
   - Nothing writes to this channel, causing permanent hang
   - **Fix**: Remove defer close and blocking read; let informer run continuously

2. **No Initial Sync on Startup** (main.go:60-70)
   - Periodic ticker waits for first interval before syncing (15 minutes default)
   - New deployments won't sync secrets until first tick
   - **Fix**: Call SyncSecrets() once before starting ticker

3. **Inefficient Namespace Watch Handler** (pkg/k8s/secret.go:197)
   - When ONE namespace is created, syncs secrets to ALL namespaces
   - Wasteful and slow for large clusters
   - **Fix**: Create syncSecretsToNamespace() for targeted single-namespace sync

4. **Inconsistent Namespace Exclusion Logic** (pkg/k8s/secret.go:145)
   - Line 145: checks if label VALUE is non-empty `ns.Labels[excludeNamespaceLabel] != ""`
   - Lines 38, 190: correctly check if label KEY exists
   - Bug causes excluded namespaces with empty label values to be synced
   - **Fix**: Use consistent existence check everywhere

5. **Missing /readyz Endpoint** (Helm deployment + pkg/metrics/prometheus.go)
   - Deployment readiness probe expects `/readyz` (charts/push-to-k8s/templates/deployment.yaml:50)
   - Only `/healthz` is implemented
   - Pods will never become ready, causing deployment failures
   - **Fix**: Add /readyz endpoint or change probe to use /healthz

6. **Empty Secrets Treated as Error** (pkg/k8s/secret.go:25-27)
   - Returns error when no source secrets exist
   - "No labeled secrets" is a valid initial state
   - **Fix**: Return empty slice with log message instead of error

### High Priority Issues

7. **Metrics Never Updated**
   - SyncMetrics() function exists but is never called
   - All Prometheus metrics remain at 0
   - **Fix**: Add periodic SyncMetrics() call in goroutine

8. **Source Labels Copied to Targets** (pkg/k8s/secret.go:55, 68)
   - DeepCopy() copies `push-to-k8s=source` label to target secrets
   - Makes it impossible to identify true source secrets
   - **Fix**: Remove source label from copied secrets before create/update

9. **No Graceful Shutdown**
   - Goroutines run forever with no cancellation mechanism
   - Kubernetes termination may leave incomplete syncs
   - **Fix**: Implement context-based cancellation and SIGTERM handling

10. **context.TODO() Used Everywhere**
    - No timeouts on Kubernetes API calls
    - Calls could hang indefinitely
    - **Fix**: Use context.WithTimeout(context.Background(), 30*time.Second) for all API calls

### Medium Priority Issues

11. **Redundant Fatalf + Return Pattern** (pkg/k8s/CreateClusterConnection.go:28-29, 36-37, 45-46)
    - Code calls logger.Fatalf() then returns error
    - Fatalf exits immediately, return is unreachable
    - **Fix**: Either return error OR call Fatalf, not both

12. **Unused Global Variable** (pkg/config/config.go:19)
    - CFG variable is assigned but never used
    - **Fix**: Remove it or document its purpose for external access

13. **Duplicate DEBUG Logic**
    - SetupLogging() reads DEBUG env var directly (pkg/logging/logging.go:15)
    - Config package also reads it
    - **Fix**: Pass config.Debug to SetupLogging() for single source of truth

14. **No Input Validation**
    - MetricsPort could be 0, negative, or > 65535
    - SyncInterval could be 0 or negative
    - **Fix**: Add validation with sensible bounds (port: 1-65535, interval: 1-1440)

15. **RBAC Check Required**
    - Verify ClusterRole has minimal required permissions
    - Current: namespaces (list/get/watch), secrets (list/get/watch/create/update)
    - This appears correct (not using cluster-admin)

## Testing

### Current State
- 0% test coverage
- No unit tests exist
- No integration tests exist

### Testing Strategy

**Unit Tests Required:**
1. **pkg/k8s/secret_test.go**
   - TestEqualByteMaps (equal, different lengths, different values, empty)
   - TestEqualStringMaps (same test cases)
   - TestCompareSecrets (identical, different data, different stringdata, metadata only)
   - TestGetSourceSecrets (success, no secrets, API error)
   - TestSyncSecretToNamespace (create new, update existing, skip identical, exclude namespace)

2. **pkg/config/config_test.go**
   - TestLoadConfigFromEnv (all defaults, all custom, mixed)
   - TestParseEnvInt (valid, invalid format, negative, missing)
   - TestParseEnvBool (true, false, missing, invalid)
   - TestGetEnvOrDefault

3. **pkg/k8s/CreateClusterConnection_test.go**
   - Test with mock kubeconfig
   - Test in-cluster config fallback

**Integration Tests (pkg/k8s/integration_test.go):**
- Use k8s.io/client-go/kubernetes/fake for fake clientset
- Test full sync workflow with multiple namespaces
- Test namespace watch triggers sync
- Test exclusion label behavior

**Coverage Goals:**
- Core logic (secret.go, config.go): 80%+ coverage
- Comparison functions: 100% coverage (critical for correctness)
- Overall project: 70%+ coverage

**Running Tests:**
```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run specific package
go test ./pkg/k8s -v

# Run specific test
go test ./pkg/k8s -run TestCompareSecrets -v
```
