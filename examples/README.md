# Push-to-K8s Examples

This directory contains example Kubernetes manifests for testing and demonstrating the push-to-k8s controller.

## Files

- **source-namespace.yaml**: Creates the source namespace where secrets will be labeled for synchronization
- **test-secret.yaml**: Example secrets with the `push-to-k8s=source` label (registry credentials, API keys, TLS cert)
- **excluded-namespace.yaml**: Example namespace with exclusion label that won't receive synced secrets
- **test-deployment.yaml**: Test deployments that consume the synced secrets

## Quick Start

### 1. Deploy the Controller

First, ensure the push-to-k8s controller is deployed:

```bash
kubectl apply -f ../deploy.yaml
```

Verify the controller is running:

```bash
kubectl get pods -n push-to-k8s
kubectl logs -n push-to-k8s deployment/push-to-k8s -f
```

### 2. Create Source Namespace and Secrets

```bash
# Create the source namespace
kubectl apply -f source-namespace.yaml

# Create example secrets with the push-to-k8s=source label
kubectl apply -f test-secret.yaml
```

### 3. Verify Secret Synchronization

The controller will automatically sync the labeled secrets to all other namespaces:

```bash
# List secrets in the source namespace
kubectl get secrets -n push-to-k8s --show-labels

# Check if secrets were synced to other namespaces
kubectl get secrets -n default
kubectl get secrets -n kube-system

# Verify secret data matches
kubectl get secret registry-credentials -n push-to-k8s -o yaml
kubectl get secret registry-credentials -n default -o yaml
```

### 4. Test Secret Exclusion (Optional)

Create a namespace that won't receive synced secrets:

```bash
# Apply the excluded namespace
kubectl apply -f excluded-namespace.yaml

# Verify the namespace has the exclusion label
kubectl get namespace no-sync-namespace --show-labels

# Check that secrets were NOT synced to this namespace
kubectl get secrets -n no-sync-namespace
```

### 5. Deploy Test Application

Deploy an application that uses the synced secrets:

```bash
# Apply the test deployment
kubectl apply -f test-deployment.yaml

# Check the deployment status
kubectl get pods -n test-app

# View the logs to see secret access
kubectl logs -n test-app deployment/secret-consumer

# You should see output confirming all secrets are mounted and accessible
```

## Testing Scenarios

### Test 1: Verify New Namespace Gets Secrets

```bash
# Create a new namespace
kubectl create namespace dynamic-test

# Wait a few seconds for the controller to detect the new namespace
sleep 5

# Verify secrets were synced
kubectl get secrets -n dynamic-test
```

### Test 2: Verify Secret Updates Propagate

```bash
# Update a secret in the source namespace
kubectl patch secret registry-credentials -n push-to-k8s \
  -p '{"stringData":{"username":"newuser"}}'

# Wait for the sync interval (default 15 minutes) or restart the controller
kubectl rollout restart deployment/push-to-k8s -n push-to-k8s

# Verify the update propagated to other namespaces
kubectl get secret registry-credentials -n default -o jsonpath='{.data.username}' | base64 -d
```

### Test 3: Verify Label Removal

```bash
# Check that synced secrets don't have the source label
kubectl get secret registry-credentials -n push-to-k8s --show-labels
# Should show: push-to-k8s=source

kubectl get secret registry-credentials -n default --show-labels
# Should NOT show push-to-k8s label
```

### Test 4: Verify Exclusion Label Works

```bash
# Add exclusion label to an existing namespace
kubectl label namespace kube-public push-to-k8s-exclude=true

# Restart controller to trigger full sync
kubectl rollout restart deployment/push-to-k8s -n push-to-k8s

# Verify secrets are removed from excluded namespace
kubectl get secrets -n kube-public | grep registry-credentials
# Should return no results
```

## Cleanup

Remove all example resources:

```bash
# Delete test deployment and namespace
kubectl delete -f test-deployment.yaml

# Delete excluded namespace
kubectl delete -f excluded-namespace.yaml

# Delete test secrets
kubectl delete -f test-secret.yaml

# Delete source namespace (will also clean up synced secrets eventually)
kubectl delete -f source-namespace.yaml
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

3. Verify controller has proper RBAC permissions:
   ```bash
   kubectl auth can-i create secrets --as=system:serviceaccount:push-to-k8s:push-to-k8s -n default
   ```

### Controller Not Starting

1. Check the deployment:
   ```bash
   kubectl describe deployment -n push-to-k8s push-to-k8s
   ```

2. Check for RBAC issues:
   ```bash
   kubectl get clusterrole push-to-k8s
   kubectl get clusterrolebinding push-to-k8s
   ```

3. Verify the NAMESPACE environment variable is set:
   ```bash
   kubectl get deployment -n push-to-k8s push-to-k8s -o jsonpath='{.spec.template.spec.containers[0].env}'
   ```

## Metrics

Check controller metrics:

```bash
# Port-forward to metrics endpoint
kubectl port-forward -n push-to-k8s deployment/push-to-k8s 9090:9090

# In another terminal, check metrics
curl http://localhost:9090/metrics | grep k8s_

# Check specific metrics
curl http://localhost:9090/metrics | grep k8s_namespace_synced_total
curl http://localhost:9090/metrics | grep k8s_source_secrets_total
```

## Additional Resources

- [Main README](../README.md) - Full documentation
- [Deployment Manifest](../deploy.yaml) - Controller deployment
- [Project Repository](https://github.com/supporttools/push-to-k8s)
