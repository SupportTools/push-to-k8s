# Release Notes: v1.1.2

**Release Date**: October 30, 2025
**Type**: Patch Release (Critical Bug Fix)

---

## 🐛 Critical Bug Fix: Secret Update Failures

This patch release fixes a **critical UID mismatch bug** that prevented secret updates from working, causing all secret synchronization operations to fail after the initial creation.

---

## What Was Fixed?

### UID Mismatch in Secret Updates

**Issue**: Secret update operations were failing with Kubernetes storage errors:

```
StorageError: invalid object, Code: 4
AdditionalErrorMsg: Precondition failed: UID in precondition: xxx, UID in object meta: yyy
```

**Root Cause**: The `DeepCopy()` method was copying the **source secret's UID** into the update object, but Kubernetes requires that the UID must match the existing target secret's UID. This caused every update attempt to be rejected.

**Impact**:
- ✗ Initial secret creation worked
- ✗ Secret updates failed completely
- ✗ Changed secrets could not be synchronized across namespaces
- ✗ The real-time watcher introduced in v1.1.0 was unable to propagate changes

**Fix**:

1. **Update Path** (pkg/k8s/secret.go:64-96)
   - Now modifies the existing target secret object directly instead of copying from source
   - Preserves target's UID, ResourceVersion, and all Kubernetes-managed metadata
   - Updates only the data fields, labels (excluding source label), and annotations
   - Ensures proper ownership and metadata consistency

2. **Create Path** (pkg/k8s/secret.go:99-121)
   - Clears all metadata fields after DeepCopy to prevent conflicts
   - Removes UID, CreationTimestamp, Generation, and ManagedFields
   - Allows Kubernetes to generate fresh identifiers for new secrets

---

## Technical Details

### Changes

**File**: `pkg/k8s/secret.go`

**Update Logic (lines 64-96):**
```go
// OLD (Broken):
sourceSecretCopy := sourceSecret.DeepCopy()  // Copies source UID!
sourceSecretCopy.ResourceVersion = existingSecret.ResourceVersion
clientset.CoreV1().Secrets(namespace).Update(ctx, sourceSecretCopy, ...)

// NEW (Fixed):
existingSecret.Data = sourceSecret.Data
existingSecret.StringData = sourceSecret.StringData
existingSecret.Type = sourceSecret.Type
// Copy labels and annotations...
clientset.CoreV1().Secrets(namespace).Update(ctx, existingSecret, ...)
```

**Create Logic (lines 99-121):**
```go
sourceSecretCopy := sourceSecret.DeepCopy()
sourceSecretCopy.Namespace = namespace
// NEW: Clear all metadata
sourceSecretCopy.UID = ""
sourceSecretCopy.CreationTimestamp = metav1.Time{}
sourceSecretCopy.Generation = 0
sourceSecretCopy.ManagedFields = nil
```

### Why This Matters

In Kubernetes, every resource has a unique UID that's assigned at creation time. When updating a resource:
- The UID in the update request **must match** the existing resource's UID
- Kubernetes uses this as a safety check to prevent accidental overwrites
- DeepCopy() was copying the wrong UID, causing all updates to fail

---

## Upgrade Instructions

### Helm Chart Upgrade

```bash
helm upgrade push-to-k8s supporttools/push-to-k8s \
  --version 1.1.2 \
  --namespace push-to-k8s
```

### Docker Image

```bash
docker pull supporttools/push-to-k8s:v1.1.2
# or
docker pull supporttools/push-to-k8s:latest
```

### Verification

After upgrading, verify secret synchronization is working:

```bash
# Watch the logs
kubectl logs -f deployment/push-to-k8s -n push-to-k8s

# You should see:
# ✓ "Updated secret <name> in namespace <namespace>" (no errors)
# ✓ No "StorageError" or "Precondition failed" messages
```

---

## Who Should Upgrade?

**Priority**: CRITICAL

- ✅ **All v1.1.0 and v1.1.1 users MUST upgrade immediately**
- ✅ **Critical for production deployments** where secret updates are required
- ✅ **Especially urgent** if you're using the real-time watcher feature from v1.1.0

**Without this fix:**
- Secrets created before v1.1.0 continue to work but cannot be updated
- Any secret changes in the source namespace will NOT propagate
- The controller will log continuous update failures

---

## Changelog

### Fixed
- **Critical**: Fixed UID mismatch causing all secret update operations to fail with "Precondition failed: UID" errors (pkg/k8s/secret.go:64-121)
- **Update path**: Now preserves target secret's UID and Kubernetes-managed metadata correctly
- **Create path**: Now clears all metadata fields after DeepCopy to prevent conflicts

### Changed
- Secret updates now modify existing secret objects instead of copying from source
- Metadata handling improved to maintain proper Kubernetes resource ownership

---

## Version History

- **v1.1.2** (Oct 30, 2025) - Fix UID mismatch preventing secret updates
- **v1.1.1** (Oct 30, 2025) - Fix nil pointer panic on startup
- **v1.1.0** (Oct 29, 2025) - Add real-time secret synchronization
- **v1.0.0** - Initial release with periodic sync

---

## Known Issues

None at this time. This release specifically addresses the critical secret update failure discovered in v1.1.0/v1.1.1.

---

## Support

For issues, questions, or feature requests:
- **GitHub Issues**: https://github.com/supporttools/push-to-k8s/issues
- **Maintainer**: mattmattox (mmattox@support.tools)

---

## Full Changelog

See the [commit history](https://github.com/supporttools/push-to-k8s/compare/v1.1.1...v1.1.2) for complete details.
