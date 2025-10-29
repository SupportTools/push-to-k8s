# Quick Start

Get Push-to-K8s up and running in 5 minutes.

---

## Step 1: Install Push-to-K8s

```bash
kubectl apply -f https://raw.githubusercontent.com/supporttools/push-to-k8s/main/deploy.yaml
```

**Verify installation:**
```bash
kubectl get pods -n push-to-k8s
```

Wait for the pod to show `Running` status.

📖 **Detailed instructions**: [Installation Guide](installation.md)

---

## Step 2: Create a Secret

Create a test secret in the `push-to-k8s` namespace:

```bash
kubectl create secret generic test-credentials \
  --from-literal=username=admin \
  --from-literal=password=changeme \
  -n push-to-k8s
```

---

## Step 3: Label the Secret

Add the `push-to-k8s=source` label to mark it for synchronization:

```bash
kubectl label secret test-credentials push-to-k8s=source -n push-to-k8s
```

**Expected output:**
```
secret/test-credentials labeled
```

---

## Step 4: Verify Synchronization

Check that the secret was synced to other namespaces:

```bash
# List the secret across all namespaces
kubectl get secrets test-credentials --all-namespaces
```

**Expected output:**
```
NAMESPACE      NAME              TYPE     DATA   AGE
push-to-k8s    test-credentials  Opaque   2      30s
default        test-credentials  Opaque   2      5s
kube-public    test-credentials  Opaque   2      5s
other-ns       test-credentials  Opaque   2      5s
```

The secret should appear in all namespaces (except those with exclusion labels).

---

## Step 5: Verify Secret Data

Confirm the data matches across namespaces:

```bash
# Check source secret
kubectl get secret test-credentials -n push-to-k8s -o yaml

# Check synced secret
kubectl get secret test-credentials -n default -o yaml
```

The `data` fields should be identical.

---

## What Happened?

When you labeled the secret with `push-to-k8s=source`:

1. ✅ Push-to-K8s detected the labeled secret
2. ✅ Found all namespaces in the cluster
3. ✅ Created a copy of the secret in each namespace
4. ✅ Removed the `push-to-k8s=source` label from copies (only source has the label)
5. ✅ Started monitoring for changes

---

## Try It: Update the Secret

Update the source secret and watch it sync:

```bash
# Update the password
kubectl create secret generic test-credentials \
  --from-literal=username=admin \
  --from-literal=password=newsecurepass \
  -n push-to-k8s \
  --dry-run=client -o yaml | kubectl apply -f -

# Re-label after update (labels don't persist through updates)
kubectl label secret test-credentials push-to-k8s=source -n push-to-k8s

# Wait for sync interval (default: 15 minutes) or restart controller
kubectl rollout restart deployment/push-to-k8s -n push-to-k8s

# Verify update propagated
kubectl get secret test-credentials -n default -o jsonpath='{.data.password}' | base64 -d
```

---

## Try It: Create a New Namespace

Create a new namespace and watch the secret appear immediately:

```bash
# Create new namespace
kubectl create namespace test-new-ns

# Wait a few seconds
sleep 5

# Check if secret synced
kubectl get secret test-credentials -n test-new-ns
```

The secret should appear automatically! 🎉

---

## Common Use Cases

### Registry Credentials

Distribute Docker registry credentials to all namespaces:

```bash
kubectl create secret docker-registry regcred \
  --docker-server=registry.example.com \
  --docker-username=user \
  --docker-password=pass \
  -n push-to-k8s

kubectl label secret regcred push-to-k8s=source -n push-to-k8s
```

Now every namespace can pull images from your private registry.

### TLS Certificates

Share wildcard certificates across all namespaces:

```bash
kubectl create secret tls wildcard-cert \
  --cert=path/to/cert.pem \
  --key=path/to/key.pem \
  -n push-to-k8s

kubectl label secret wildcard-cert push-to-k8s=source -n push-to-k8s
```

### API Keys

Distribute API keys to all applications:

```bash
kubectl create secret generic api-keys \
  --from-literal=stripe-key=sk_live_xxx \
  --from-literal=sendgrid-key=SG.xxx \
  -n push-to-k8s

kubectl label secret api-keys push-to-k8s=source -n push-to-k8s
```

---

## Cleanup

Remove the test secret:

```bash
# Delete from source (won't auto-delete from targets)
kubectl delete secret test-credentials -n push-to-k8s

# Manually delete from targets if needed
kubectl delete secret test-credentials --all-namespaces
```

---

## Next Steps

✅ **You're now syncing secrets automatically!**

**Learn more:**
- **[Configuration](configuration.md)** - Customize sync interval, exclude namespaces
- **[Basic Usage](../guides/basic-usage.md)** - Detailed workflows and patterns
- **[Monitoring](../guides/monitoring.md)** - Prometheus metrics and observability
- **[Troubleshooting](../guides/troubleshooting.md)** - Common issues and solutions

---

## Need Help?

- 📚 [Full Documentation](../index.md)
- 🐛 [Report Issues](https://github.com/supporttools/push-to-k8s/issues)
- 💬 [Ask Questions](https://github.com/supporttools/push-to-k8s/discussions)
