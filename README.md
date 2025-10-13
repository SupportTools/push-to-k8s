# Push-to-K8s

Push-to-K8s is a Kubernetes controller that automatically synchronizes secrets from a source namespace to all other namespaces in your cluster. It watches for new namespaces and keeps secrets in sync across your entire cluster.

## Features

- **Automatic Synchronization**: Syncs labeled secrets from a source namespace to all other namespaces
- **Namespace Watch**: Automatically syncs secrets to newly created namespaces in real-time
- **Selective Exclusion**: Skip specific namespaces using labels
- **Prometheus Metrics**: Built-in monitoring and observability
- **Graceful Shutdown**: Proper cleanup on termination signals
- **Health Checks**: Liveness and readiness endpoints
- **Production Ready**: Comprehensive test coverage and input validation

## Use Cases

- Distribute registry credentials across all namespaces
- Share TLS certificates cluster-wide
- Distribute API keys and tokens to all applications
- Maintain consistent secrets across development/staging environments

## Prerequisites

- **Kubernetes**: Version 1.19 or higher
- **RBAC**: ClusterRole permissions to read/write secrets and namespaces
- **Go**: Version 1.21+ (for building from source)

## Quick Start

### 1. Create Source Namespace

```bash
kubectl create namespace push-to-k8s
```

### 2. Deploy the Controller

```bash
kubectl apply -f deploy.yaml
```

### 3. Create a Secret with the Label

```bash
kubectl create secret generic my-secret \
  --from-literal=key1=value1 \
  --from-literal=key2=value2 \
  -n push-to-k8s

kubectl label secret my-secret push-to-k8s=source -n push-to-k8s
```

### 4. Verify Synchronization

```bash
# Check that the secret was synced to other namespaces
kubectl get secrets my-secret -n default
kubectl get secrets my-secret -n kube-system
```

## Configuration

The controller is configured using environment variables:

| Variable | Description | Default | Valid Range |
|----------|-------------|---------|-------------|
| `NAMESPACE` | Source namespace for secrets (required) | - | Any valid namespace name |
| `DEBUG` | Enable debug logging | `false` | `true` or `false` |
| `METRICS_PORT` | Port for Prometheus metrics server | `9090` | 1-65535 |
| `SYNC_INTERVAL` | Sync interval in minutes | `15` | 1-1440 (24 hours max) |
| `EXCLUDE_NAMESPACE_LABEL` | Label key to exclude namespaces | `""` | Any valid label key |

### Configuration Examples

**Basic Configuration:**
```yaml
env:
  - name: NAMESPACE
    value: "push-to-k8s"
```

**Debug Mode:**
```yaml
env:
  - name: NAMESPACE
    value: "push-to-k8s"
  - name: DEBUG
    value: "true"
```

**Custom Sync Interval:**
```yaml
env:
  - name: NAMESPACE
    value: "push-to-k8s"
  - name: SYNC_INTERVAL
    value: "30"  # Sync every 30 minutes
```

**Exclude Specific Namespaces:**
```yaml
env:
  - name: NAMESPACE
    value: "push-to-k8s"
  - name: EXCLUDE_NAMESPACE_LABEL
    value: "push-to-k8s-exclude"
```

Then label namespaces to exclude:
```bash
kubectl label namespace kube-system push-to-k8s-exclude=true
```

## Deployment

### Using Kubernetes Manifests

1. **Review the deployment manifest:**
   ```bash
   cat deploy.yaml
   ```

2. **Customize environment variables** in the deployment if needed

3. **Apply the manifest:**
   ```bash
   kubectl apply -f deploy.yaml
   ```

4. **Verify deployment:**
   ```bash
   kubectl get pods -n push-to-k8s
   kubectl logs -n push-to-k8s deployment/push-to-k8s
   ```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/supporttools/push-to-k8s.git
cd push-to-k8s

# Build the binary
go build -o push-to-k8s main.go

# Build Docker image
docker build -t push-to-k8s:latest .
```

### Running Tests

```bash
# Run unit tests
go test ./pkg/...

# Run tests with coverage
go test -cover ./pkg/...

# Run integration tests
go test -v ./pkg/k8s/...
```

## How It Works

### Secret Labeling

Secrets in the source namespace must have the label `push-to-k8s=source`:

```bash
kubectl label secret my-secret push-to-k8s=source -n push-to-k8s
```

Only labeled secrets are synchronized. This prevents accidental syncing of all secrets.

### Synchronization Process

1. **Initial Sync**: On startup, syncs all labeled secrets to all namespaces
2. **Periodic Sync**: Checks for changes every `SYNC_INTERVAL` minutes
3. **Namespace Watch**: Automatically syncs to newly created namespaces
4. **Update Detection**: Only updates secrets that have changed

### Label Removal

The `push-to-k8s=source` label is **removed** from synced secrets in target namespaces. This ensures only the source secret has the label, making it easy to identify the source of truth.

### Exclusion Behavior

- Source namespace is automatically excluded (never syncs to itself)
- Namespaces with the exclusion label are skipped
- System namespaces (kube-system, kube-public) can be excluded using labels

## Verifying the Controller

### Check Pod Status

```bash
kubectl get pods -n push-to-k8s
```

Expected output:
```
NAME                            READY   STATUS    RESTARTS   AGE
push-to-k8s-xxxxxxxxxx-xxxxx   1/1     Running   0          5m
```

### Check Logs

```bash
kubectl logs -n push-to-k8s deployment/push-to-k8s -f
```

Successful startup logs:
```
INFO[0000] Debug mode disabled
INFO[0000] Successfully connected to Kubernetes cluster using in-cluster configuration
INFO[0000] Starting Prometheus metrics server at :9090
INFO[0000] Performing initial secret sync on startup
INFO[0001] Namespace watcher started successfully
```

### Verify Secret Synchronization

```bash
# Create a test secret in source namespace
kubectl create secret generic test-sync \
  --from-literal=test=value \
  -n push-to-k8s

kubectl label secret test-sync push-to-k8s=source -n push-to-k8s

# Wait a few seconds, then check if it synced to another namespace
kubectl get secret test-sync -n default

# Verify the data matches
kubectl get secret test-sync -n push-to-k8s -o yaml
kubectl get secret test-sync -n default -o yaml
```

## Monitoring and Metrics

The controller exposes Prometheus metrics on port 9090 (configurable):

### Available Endpoints

- `http://<pod-ip>:9090/metrics` - Prometheus metrics
- `http://<pod-ip>:9090/healthz` - Health check
- `http://<pod-ip>:9090/readyz` - Readiness check
- `http://<pod-ip>:9090/version` - Version information

### Key Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `k8s_connection_success_total` | Counter | Successful K8s client connections |
| `k8s_connection_failures_total` | Counter | Failed K8s client connections |
| `k8s_namespace_total` | Gauge | Total namespaces in cluster |
| `k8s_namespace_synced_total` | Gauge | Namespaces with all secrets synced |
| `k8s_namespace_not_synced_total` | Gauge | Namespaces missing some secrets |
| `k8s_source_secrets_total` | Gauge | Labeled secrets in source namespace |
| `k8s_managed_secrets_total` | Gauge | Total secrets managed by controller |

### Example Prometheus Query

```promql
# Check sync status
k8s_namespace_synced_total / k8s_namespace_total * 100

# Secrets per namespace
k8s_source_secrets_total
```

## Troubleshooting

### Controller Pod Not Starting

**Problem**: Pod is in `CrashLoopBackOff` or `Error` state

**Solution**:
```bash
# Check pod events
kubectl describe pod -n push-to-k8s <pod-name>

# Check logs
kubectl logs -n push-to-k8s <pod-name>
```

**Common Issues**:
- Missing RBAC permissions
- Invalid NAMESPACE configuration
- Network connectivity issues

### Secrets Not Syncing

**Problem**: Labeled secrets are not appearing in target namespaces

**Diagnostics**:
```bash
# 1. Verify the secret has the correct label
kubectl get secret <secret-name> -n push-to-k8s --show-labels

# 2. Check controller logs for errors
kubectl logs -n push-to-k8s deployment/push-to-k8s | grep -i error

# 3. Verify RBAC permissions
kubectl auth can-i create secrets --as=system:serviceaccount:push-to-k8s:push-to-k8s -n default

# 4. Check metrics for sync status
kubectl port-forward -n push-to-k8s deployment/push-to-k8s 9090:9090
curl http://localhost:9090/metrics | grep k8s_namespace
```

**Common Causes**:
- Secret missing `push-to-k8s=source` label
- Target namespace has exclusion label
- RBAC permissions not configured
- Controller not running

### Specific Namespace Not Receiving Secrets

**Problem**: One namespace is not getting synced secrets

**Check**:
```bash
# Check if namespace has exclusion label
kubectl get namespace <namespace-name> --show-labels

# Remove exclusion label if present
kubectl label namespace <namespace-name> push-to-k8s-exclude-
```

### High Resource Usage

**Problem**: Controller using too much CPU/memory

**Solution**:
```bash
# Increase sync interval to reduce frequency
kubectl set env deployment/push-to-k8s -n push-to-k8s SYNC_INTERVAL=60

# Add resource limits to deployment
kubectl set resources deployment/push-to-k8s -n push-to-k8s \
  --limits=cpu=200m,memory=256Mi \
  --requests=cpu=100m,memory=128Mi
```

### Secret Updates Not Propagating

**Problem**: Changes to source secret not updating target namespaces

**Diagnosis**:
```bash
# Check last sync time in logs
kubectl logs -n push-to-k8s deployment/push-to-k8s | tail -20

# Manually trigger sync by restarting controller
kubectl rollout restart deployment/push-to-k8s -n push-to-k8s
```

**Note**: Changes propagate on the next sync interval. Wait for `SYNC_INTERVAL` minutes or restart the controller.

### Connection Errors

**Problem**: "Failed to connect to Kubernetes cluster"

**Solution**:
- Ensure ServiceAccount has proper RBAC permissions
- Verify the controller can reach the Kubernetes API server
- Check network policies and firewall rules

### Configuration Validation Warnings

**Problem**: Logs show "WARNING: METRICS_PORT value out of valid range"

**Solution**: These are non-fatal warnings. The controller uses safe defaults when invalid values are provided. Update your configuration to remove the warnings:

```yaml
env:
  - name: METRICS_PORT
    value: "9090"  # Must be 1-65535
  - name: SYNC_INTERVAL
    value: "15"    # Must be 1-1440 minutes
```

## Security Considerations

### RBAC Permissions

The controller requires ClusterRole permissions to:
- **List/Watch Namespaces**: Monitor for new namespaces
- **Get/List/Create/Update Secrets**: Sync secrets across namespaces

Review the RBAC configuration in `deploy.yaml` before deploying to production.

### Secret Management

- Only label secrets that should be cluster-wide as `push-to-k8s=source`
- Sensitive secrets (database passwords, private keys) should be carefully considered
- Use namespace exclusion labels for namespaces that shouldn't receive all secrets
- Consider using separate source namespaces for different security tiers

### Network Policies

If using network policies, ensure the controller pod can:
- Reach the Kubernetes API server
- Be reached by Prometheus (if scraping metrics)

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Push-to-K8s Controller                │
│                                                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │   Periodic   │  │  Namespace   │  │   Metrics    │  │
│  │   Syncer     │  │   Watcher    │  │   Server     │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  │
│         │                  │                  │           │
│         └──────────┬───────┘                  │           │
│                    │                          │           │
│              ┌─────▼─────┐              ┌─────▼─────┐   │
│              │   Secret  │              │ Prometheus│   │
│              │   Sync    │              │  Metrics  │   │
│              │   Logic   │              └───────────┘   │
│              └─────┬─────┘                              │
└────────────────────┼──────────────────────────────────┘
                     │
                     ▼
          ┌──────────────────┐
          │ Kubernetes API    │
          │ Server            │
          └───────┬───────────┘
                  │
      ┌───────────┼───────────┐
      │           │           │
┌─────▼─────┐ ┌──▼───────┐ ┌─▼──────────┐
│ Namespace │ │Namespace │ │ Namespace  │
│ (source)  │ │ (target) │ │  (target)  │
│           │ │          │ │            │
│ ┌───────┐ │ │┌───────┐ │ │┌───────┐  │
│ │Secret │ │ ││Secret │ │ ││Secret │  │
│ │(label)│ │ ││(copy) │ │ ││(copy) │  │
│ └───────┘ │ │└───────┘ │ │└───────┘  │
└───────────┘ └──────────┘ └────────────┘
```

## Development

### Project Structure

```
push-to-k8s/
├── main.go                      # Application entry point
├── deploy.yaml                  # Kubernetes deployment manifest
├── pkg/
│   ├── config/                  # Configuration management
│   │   ├── config.go
│   │   └── config_test.go
│   ├── k8s/                     # Kubernetes operations
│   │   ├── secret.go
│   │   ├── secret_test.go
│   │   ├── integration_test.go
│   │   └── CreateClusterConnection.go
│   ├── logging/                 # Logging setup
│   │   └── logging.go
│   ├── metrics/                 # Prometheus metrics
│   │   └── prometheus.go
│   └── version/                 # Version information
│       └── version.go
└── README.md
```

### Running Locally

```bash
# Set required environment variables
export NAMESPACE=push-to-k8s
export DEBUG=true
export KUBECONFIG=~/.kube/config

# Run the controller
go run main.go
```

### Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Submit a pull request

## Changelog

See [GitHub Releases](https://github.com/supporttools/push-to-k8s/releases) for version history and release notes.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/supporttools/push-to-k8s/issues)
- **Discussions**: [GitHub Discussions](https://github.com/supporttools/push-to-k8s/discussions)
