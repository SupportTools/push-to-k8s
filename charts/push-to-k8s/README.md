# Push-to-K8s Helm Chart

A Kubernetes controller that automatically synchronizes labeled secrets from a source namespace to all other namespaces in the cluster.

## Overview

Push-to-K8s is a lightweight Go-based controller that watches for secrets labeled with `push-to-k8s=source` in a designated namespace and automatically syncs them to all other namespaces (excluding system namespaces and those with exclusion labels). This is ideal for distributing common secrets like registry credentials, API keys, and TLS certificates across your entire cluster.

## Features

- **Automatic Secret Synchronization**: Syncs labeled secrets to all namespaces
- **Real-time Namespace Watching**: Automatically syncs secrets to newly created namespaces
- **Smart Update Detection**: Only updates secrets when data changes
- **Namespace Exclusion**: Label namespaces to exclude them from receiving secrets
- **Prometheus Metrics**: Built-in metrics for monitoring controller health
- **Health Endpoints**: `/healthz` and `/readyz` endpoints for Kubernetes probes
- **Graceful Shutdown**: Proper signal handling and cleanup

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+

## Installation

### Add the Helm Repository (if available)

```bash
helm repo add push-to-k8s https://supporttools.github.io/push-to-k8s
helm repo update
```

### Install from Local Chart

```bash
# From the repository root
helm install push-to-k8s ./charts/push-to-k8s --namespace push-to-k8s --create-namespace
```

### Install with Custom Values

```bash
helm install push-to-k8s ./charts/push-to-k8s \
  --namespace push-to-k8s \
  --create-namespace \
  --set settings.debug=true \
  --set settings.SyncInterval=10
```

## Configuration

### Key Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `settings.debug` | Enable debug logging | `false` |
| `settings.metrics.enabled` | Enable Prometheus metrics | `true` |
| `settings.metrics.port` | Metrics server port | `9090` |
| `settings.ExcludeNamespaceLabel` | Label to exclude namespaces | `""` |
| `settings.SyncInterval` | Sync interval in minutes | `15` |
| `replicaCount` | Number of controller replicas | `1` |
| `image.repository` | Container image repository | `cube8021/push-to-k8s` |
| `image.tag` | Container image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `Always` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `resources.limits.cpu` | CPU limit | `200m` |
| `resources.limits.memory` | Memory limit | `256Mi` |

### Example values.yaml

```yaml
settings:
  debug: false
  metrics:
    enabled: true
    port: 9090
  ExcludeNamespaceLabel: "push-to-k8s-exclude"
  SyncInterval: 15

image:
  repository: cube8021/push-to-k8s
  tag: "v1.0.0"
  pullPolicy: IfNotPresent

resources:
  limits:
    cpu: 200m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi

nodeSelector:
  kubernetes.io/os: linux
```

## Usage

### 1. Label Secrets for Synchronization

Label any secret in the controller's namespace with `push-to-k8s=source`:

```bash
kubectl create secret generic my-registry-credentials \
  --from-literal=username=myuser \
  --from-literal=password=mypass \
  --namespace push-to-k8s

kubectl label secret my-registry-credentials push-to-k8s=source -n push-to-k8s
```

### 2. Verify Secret Synchronization

```bash
# Check the source secret
kubectl get secret my-registry-credentials -n push-to-k8s --show-labels

# Verify it was synced to other namespaces
kubectl get secret my-registry-credentials -n default
kubectl get secret my-registry-credentials -n production
```

### 3. Exclude Namespaces (Optional)

Set the `ExcludeNamespaceLabel` value and label namespaces to exclude:

```bash
helm upgrade push-to-k8s ./charts/push-to-k8s \
  --set settings.ExcludeNamespaceLabel=push-to-k8s-exclude \
  --reuse-values

kubectl label namespace kube-system push-to-k8s-exclude=true
kubectl label namespace kube-public push-to-k8s-exclude=true
```

## Monitoring

### Accessing Metrics

Port-forward to the controller and access Prometheus metrics:

```bash
kubectl port-forward -n push-to-k8s deployment/push-to-k8s 9090:9090

# View all metrics
curl http://localhost:9090/metrics

# View controller-specific metrics
curl http://localhost:9090/metrics | grep k8s_
```

### Available Metrics

- `k8s_namespace_total` - Total number of namespaces in the cluster
- `k8s_namespace_synced_total` - Number of namespaces receiving synced secrets
- `k8s_namespace_not_synced_total` - Number of excluded namespaces
- `k8s_source_secrets_total` - Number of source secrets being synced
- `k8s_managed_secrets_total` - Total number of managed secrets across all namespaces
- `k8s_connection_success_total` - Kubernetes connection status

### Health Checks

```bash
# Liveness probe
curl http://localhost:9090/healthz

# Readiness probe
curl http://localhost:9090/readyz
```

## Troubleshooting

### Secrets Not Syncing

1. Verify the controller is running:
   ```bash
   kubectl get pods -n push-to-k8s
   kubectl logs -n push-to-k8s deployment/push-to-k8s
   ```

2. Check that secrets have the correct label:
   ```bash
   kubectl get secrets -n push-to-k8s --show-labels | grep push-to-k8s
   ```

3. Verify RBAC permissions:
   ```bash
   kubectl auth can-i create secrets --as=system:serviceaccount:push-to-k8s:push-to-k8s -n default
   ```

### Controller Not Starting

1. Check deployment status:
   ```bash
   kubectl describe deployment -n push-to-k8s push-to-k8s
   ```

2. Verify service account exists:
   ```bash
   kubectl get serviceaccount -n push-to-k8s
   ```

3. Check RBAC resources:
   ```bash
   kubectl get clusterrole,clusterrolebinding | grep push-to-k8s
   ```

## Upgrading

```bash
# Update chart values
helm upgrade push-to-k8s ./charts/push-to-k8s \
  --namespace push-to-k8s \
  --reuse-values \
  --set image.tag=v1.1.0
```

## Uninstalling

```bash
helm uninstall push-to-k8s --namespace push-to-k8s
```

**Note**: This will not automatically remove synced secrets from other namespaces. You can manually clean them up if needed.

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for details on contributing to this project.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](../../LICENSE) file for details.

## Support

- GitHub Issues: https://github.com/supporttools/push-to-k8s/issues
- Documentation: https://github.com/supporttools/push-to-k8s
- Maintainer: mattmattox (mmattox@support.tools)
