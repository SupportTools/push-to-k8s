# Installation

This guide covers installing Push-to-K8s on your Kubernetes cluster.

---

## Prerequisites

Before installing Push-to-K8s, ensure you have:

- **Kubernetes cluster**: Version 1.19 or higher
- **kubectl**: Configured to access your cluster
- **Cluster admin permissions**: Required to create ClusterRole and ClusterRoleBinding

### Verify Prerequisites

```bash
# Check kubectl is configured
kubectl version

# Check you have cluster admin access
kubectl auth can-i create clusterrole
```

---

## Installation Methods

### Method 1: Quick Install (Recommended)

Deploy Push-to-K8s with a single command:

```bash
kubectl apply -f https://raw.githubusercontent.com/supporttools/push-to-k8s/main/deploy.yaml
```

This installs:
- Namespace (`push-to-k8s`)
- ServiceAccount
- ClusterRole and ClusterRoleBinding (RBAC)
- Deployment
- Service (for metrics)

### Method 2: Helm Chart

> **Coming Soon**: Helm chart with customizable values

### Method 3: Custom Installation

For custom configurations, download and modify the deployment manifest:

```bash
# Download the manifest
curl -O https://raw.githubusercontent.com/supporttools/push-to-k8s/main/deploy.yaml

# Edit as needed
vim deploy.yaml

# Apply
kubectl apply -f deploy.yaml
```

---

## Verify Installation

### Check Pod Status

```bash
kubectl get pods -n push-to-k8s
```

**Expected output:**
```
NAME                           READY   STATUS    RESTARTS   AGE
push-to-k8s-xxxxxxxxxx-xxxxx   1/1     Running   0          1m
```

### Check Logs

```bash
kubectl logs -n push-to-k8s deployment/push-to-k8s
```

**Successful startup logs:**
```
INFO[0000] Debug mode disabled
INFO[0000] Successfully connected to Kubernetes cluster
INFO[0000] Starting Prometheus metrics server at :9090
INFO[0000] Performing initial secret sync on startup
INFO[0001] Namespace watcher started successfully
```

### Verify RBAC Permissions

```bash
# Check ClusterRole was created
kubectl get clusterrole push-to-k8s

# Check ClusterRoleBinding
kubectl get clusterrolebinding push-to-k8s
```

---

## Configuration Options

The controller is configured via environment variables in the Deployment.

### Basic Configuration

Edit the deployment to customize:

```bash
kubectl edit deployment -n push-to-k8s push-to-k8s
```

Common environment variables:

```yaml
env:
  - name: NAMESPACE
    value: "push-to-k8s"  # Source namespace
  - name: DEBUG
    value: "false"         # Enable debug logging
  - name: SYNC_INTERVAL
    value: "15"            # Sync interval in minutes
  - name: METRICS_PORT
    value: "9090"          # Metrics server port
  - name: EXCLUDE_NAMESPACE_LABEL
    value: ""              # Label key to exclude namespaces
```

📖 **See [Configuration Guide](configuration.md) for all available options**

---

## What Gets Installed

### Namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: push-to-k8s
```

The `push-to-k8s` namespace is the default source namespace for secrets.

### ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: push-to-k8s
  namespace: push-to-k8s
```

The controller runs with this service account.

### RBAC Permissions

The controller needs these cluster-level permissions:

**Namespaces:**
- `get`, `list`, `watch` - To discover and monitor namespaces

**Secrets:**
- `get`, `list`, `watch` - To read source secrets
- `create`, `update` - To sync secrets to target namespaces

These permissions are defined in a ClusterRole and bound via ClusterRoleBinding.

### Deployment

The controller runs as a single-replica Deployment with:
- Resource limits: 256Mi memory, 200m CPU
- Health checks: Liveness and readiness probes
- Metrics port: 9090 (exposed via Service)

### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: push-to-k8s-metrics
  namespace: push-to-k8s
spec:
  ports:
    - port: 9090
      name: metrics
```

Exposes Prometheus metrics for monitoring.

---

## Upgrading

### Upgrade to Latest Version

```bash
kubectl apply -f https://raw.githubusercontent.com/supporttools/push-to-k8s/main/deploy.yaml
```

The deployment will perform a rolling update with zero downtime.

### Upgrade to Specific Version

```bash
# Replace v1.0.0 with desired version
kubectl set image deployment/push-to-k8s \
  push-to-k8s=cube8021/push-to-k8s:v1.0.0 \
  -n push-to-k8s
```

### Verify Upgrade

```bash
# Check deployment status
kubectl rollout status deployment/push-to-k8s -n push-to-k8s

# Check version
kubectl exec -n push-to-k8s deployment/push-to-k8s -- /push-to-k8s --version
```

---

## Uninstalling

### Complete Removal

Remove Push-to-K8s and all resources:

```bash
kubectl delete -f https://raw.githubusercontent.com/supporttools/push-to-k8s/main/deploy.yaml
```

**Note**: This does NOT delete synced secrets in target namespaces. To clean up synced secrets:

```bash
# List all synced secrets (example for secret named "my-secret")
kubectl get secrets my-secret --all-namespaces

# Delete manually from each namespace
kubectl delete secret my-secret -n <namespace>
```

### Keep Namespace, Remove Controller

If you want to keep the namespace but remove the controller:

```bash
kubectl delete deployment push-to-k8s -n push-to-k8s
kubectl delete service push-to-k8s-metrics -n push-to-k8s
```

---

## Troubleshooting Installation

### Pod Won't Start

**Problem**: Pod stuck in `CrashLoopBackOff` or `Error`

**Diagnosis**:
```bash
kubectl describe pod -n push-to-k8s <pod-name>
kubectl logs -n push-to-k8s <pod-name>
```

**Common causes**:
- Missing RBAC permissions
- Invalid NAMESPACE configuration
- Image pull errors

### RBAC Permission Denied

**Problem**: Logs show "forbidden" errors

**Solution**: Verify ClusterRole and ClusterRoleBinding:
```bash
kubectl get clusterrole push-to-k8s -o yaml
kubectl get clusterrolebinding push-to-k8s -o yaml
```

Ensure the ServiceAccount is correctly bound to the ClusterRole.

### Can't Pull Image

**Problem**: `ImagePullBackOff` error

**Solution**: Check image name and registry access:
```bash
kubectl describe pod -n push-to-k8s <pod-name>
```

Verify the image exists:
- DockerHub: `cube8021/push-to-k8s:latest`
- GitHub Container Registry: `ghcr.io/supporttools/push-to-k8s:latest`

---

## Next Steps

✅ **Installation complete!**

Now you're ready to:
1. **[Quick Start Guide](quick-start.md)** - Create and sync your first secret
2. **[Configuration](configuration.md)** - Customize Push-to-K8s settings
3. **[Basic Usage](../guides/basic-usage.md)** - Learn core workflows

---

## Additional Resources

- [Configuration Reference](configuration.md)
- [Architecture Overview](../architecture/overview.md)
- [Troubleshooting Guide](../guides/troubleshooting.md)
- [GitHub Repository](https://github.com/supporttools/push-to-k8s)
