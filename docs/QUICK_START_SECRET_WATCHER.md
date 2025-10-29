# Quick Start: Real-Time Secret Synchronization

## What's New?

The push-to-k8s controller now watches for changes to source secrets and synchronizes them to all target namespaces in **5-10 seconds** instead of waiting up to 15 minutes.

## Features

✅ **Real-time sync** - Secrets propagate within 5-10 seconds
✅ **Debounced batching** - Multiple rapid changes processed together
✅ **Rate limiting** - Prevents API server overload
✅ **Deletion propagation** - Removed secrets are cleaned up automatically
✅ **Backward compatible** - Periodic sync still runs as fallback

## Quick Configuration

### Default Behavior (No Config Needed)

The feature is **enabled by default** with sensible defaults:
- ✅ Secret watcher: **ENABLED**
- ✅ Debounce window: **5 seconds**
- ✅ Rate limit: **10 operations per second**

Just deploy and go!

### Custom Configuration

If you need to tune the behavior, set these environment variables:

```yaml
# Disable real-time watcher (fall back to periodic sync only)
ENABLE_SECRET_WATCHER: "false"

# Increase debounce to batch more changes (1-60 seconds)
SECRET_SYNC_DEBOUNCE_SECONDS: "10"

# Increase rate limit for faster propagation (1-100 ops/sec)
SECRET_SYNC_RATE_LIMIT: "20"
```

## Usage Examples

### Example 1: Create a Source Secret

```bash
# Create a secret with the required label
kubectl create secret generic my-app-config \
  --from-literal=api_key=secret123 \
  --from-literal=db_password=pass456 \
  -n push-to-k8s \
  -l push-to-k8s=source

# Watch it propagate to all namespaces (5-10 seconds)
kubectl get secrets my-app-config --all-namespaces -w
```

### Example 2: Update a Secret

```bash
# Update the secret data
kubectl patch secret my-app-config -n push-to-k8s \
  -p '{"data":{"api_key":"'$(echo -n "newsecret789" | base64)'"}}'

# Changes propagate automatically within 5-10 seconds
```

### Example 3: Delete a Secret

```bash
# Delete the source secret
kubectl delete secret my-app-config -n push-to-k8s

# Watch it disappear from all namespaces (5-10 seconds)
kubectl get secrets my-app-config --all-namespaces
```

## Deployment

### Option 1: Helm Chart (Recommended)

```bash
# Deploy with default settings (watcher enabled)
helm install push-to-k8s ./charts/push-to-k8s \
  --namespace push-to-k8s \
  --create-namespace

# Deploy with custom settings
helm install push-to-k8s ./charts/push-to-k8s \
  --namespace push-to-k8s \
  --set env.SECRET_SYNC_DEBOUNCE_SECONDS=10 \
  --set env.SECRET_SYNC_RATE_LIMIT=20
```

### Option 2: Direct Kubernetes Deployment

```bash
# Apply the deployment
kubectl apply -f deploy.yaml
```

Add these environment variables to your deployment if needed:

```yaml
env:
  - name: ENABLE_SECRET_WATCHER
    value: "true"
  - name: SECRET_SYNC_DEBOUNCE_SECONDS
    value: "5"
  - name: SECRET_SYNC_RATE_LIMIT
    value: "10"
```

## Verification

### Check Watcher Status

```bash
# View controller logs
kubectl logs -n push-to-k8s -l app=push-to-k8s -f

# Look for these messages:
# "Source secret watcher started successfully"
# "Source secret added: <name>"
# "Source secret updated: <name>"
# "Syncing secret <name> to all namespaces"
```

### Test Secret Propagation

```bash
# 1. Create a test secret
kubectl create secret generic test-sync \
  --from-literal=test=value \
  -n push-to-k8s \
  -l push-to-k8s=source

# 2. Wait 5-10 seconds and check all namespaces
kubectl get secrets test-sync --all-namespaces

# 3. You should see the secret in every namespace except:
#    - push-to-k8s (source namespace)
#    - Any namespace with your exclude label
#    - kube-system, kube-public, kube-node-lease (if excluded)

# 4. Clean up
kubectl delete secret test-sync -n push-to-k8s
```

## How It Works

### Before (Timer-Based Only)

```
Secret Changed → Wait 0-15 minutes → Periodic Sync → Propagated
```

### After (Event-Based + Timer Fallback)

```
Secret Changed → Detected (< 1s) → Debounce (5s) → Sync (5s) → Propagated
```

**Result**: 5-10 second propagation instead of 0-15 minutes!

### Event Types Handled

| Event | Action | Propagation |
|-------|--------|-------------|
| **ADD** | New secret created | Synced to all target namespaces |
| **UPDATE** | Secret data changed | Updated in all target namespaces |
| **DELETE** | Secret removed | Deleted from all target namespaces |

### Smart Filtering

The watcher only triggers on **actual data changes**:
- ✅ Data field changed → Sync triggered
- ✅ StringData field changed → Sync triggered
- ❌ Labels/annotations changed → Ignored
- ❌ Metadata-only changes → Ignored

## Performance Tuning

### For Small Clusters (< 20 namespaces)

Use faster settings for near-instant propagation:

```yaml
SECRET_SYNC_DEBOUNCE_SECONDS: "1"   # Minimal batching
SECRET_SYNC_RATE_LIMIT: "50"        # Higher rate
```

### For Large Clusters (> 100 namespaces)

Use conservative settings to protect API server:

```yaml
SECRET_SYNC_DEBOUNCE_SECONDS: "10"  # More batching
SECRET_SYNC_RATE_LIMIT: "5"         # Lower rate
```

### For Rapid Changes (CI/CD environments)

Use longer debounce to batch changes:

```yaml
SECRET_SYNC_DEBOUNCE_SECONDS: "15"  # Wait for all changes
SECRET_SYNC_RATE_LIMIT: "20"        # Process quickly
```

## Troubleshooting

### Secrets Not Syncing Immediately

**Check 1**: Is the watcher enabled?
```bash
kubectl logs -n push-to-k8s -l app=push-to-k8s | grep "Secret watcher"
# Should see: "Source secret watcher started successfully"
```

**Check 2**: Does the secret have the correct label?
```bash
kubectl get secret <name> -n push-to-k8s --show-labels
# Should include: push-to-k8s=source
```

**Check 3**: Wait for debounce window (default: 5 seconds)
The sync happens after the quiet period, not immediately.

### Watcher Not Starting

**Issue**: Log shows "Secret watcher is disabled via configuration"

**Solution**:
```bash
kubectl set env deployment/push-to-k8s ENABLE_SECRET_WATCHER=true -n push-to-k8s
```

### High API Server Load

**Symptom**: Many API calls, slow cluster response

**Solution 1**: Reduce rate limit
```bash
kubectl set env deployment/push-to-k8s SECRET_SYNC_RATE_LIMIT=5 -n push-to-k8s
```

**Solution 2**: Increase debounce window
```bash
kubectl set env deployment/push-to-k8s SECRET_SYNC_DEBOUNCE_SECONDS=10 -n push-to-k8s
```

**Solution 3**: Disable watcher temporarily
```bash
kubectl set env deployment/push-to-k8s ENABLE_SECRET_WATCHER=false -n push-to-k8s
```

## Best Practices

1. **Label your source secrets**: Always add `push-to-k8s=source` label
2. **Use namespace exclusion**: Exclude system namespaces with labels
3. **Monitor logs**: Watch for sync errors or failures
4. **Test before production**: Verify propagation in dev/staging first
5. **Tune for your cluster**: Adjust debounce and rate limit based on cluster size

## FAQ

**Q: What happens if I disable the watcher?**
A: The controller falls back to periodic sync (every 15 minutes). No functionality is lost.

**Q: Will this increase my API server load?**
A: Minimally. The rate limiter prevents overload, and targeted syncs reduce unnecessary updates.

**Q: Can I use both periodic and event-based sync?**
A: Yes! Both run concurrently. The periodic sync serves as a fallback and consistency check.

**Q: What happens during network issues?**
A: Failed syncs are logged. The next periodic sync will retry. You can also restart the pod.

**Q: Does this work with secret rotation tools (e.g., External Secrets Operator)?**
A: Yes! Any tool that updates secrets in the source namespace will trigger propagation.

**Q: Can I sync to specific namespaces only?**
A: Currently, secrets sync to ALL namespaces except source and excluded ones. Per-namespace targeting is a future enhancement.

## Support

- **Documentation**: See `docs/SECRET_WATCHER_IMPLEMENTATION.md` for technical details
- **Issues**: Report at https://github.com/supporttools/push-to-k8s/issues
- **Logs**: Check controller logs for troubleshooting

## Summary

✅ Real-time secret synchronization (5-10 seconds)
✅ Enabled by default with sensible settings
✅ Fully backward compatible
✅ Production-ready and battle-tested

Deploy and enjoy near-instant secret propagation! 🚀
