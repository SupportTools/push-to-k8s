# Release Notes: v1.1.1

**Release Date**: October 30, 2025
**Type**: Patch Release (Critical Bug Fix)

---

## 🐛 Critical Bug Fix: Startup Panic

This patch release fixes a **critical nil pointer dereference panic** that could prevent the controller from starting up successfully.

---

## What Was Fixed?

### Nil Pointer Dereference in Secret Queue Processor

**Issue**: The real-time secret watcher introduced in v1.1.0 contained a race condition that could cause a panic on startup:

```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x187e054]
goroutine 72 [running]:
github.com/supporttools/push-to-k8s/pkg/k8s.processDebouncedSecretQueue
```

**Root Cause**: The debounce timer was initialized as `nil` and could be dereferenced in a `select` statement before any events arrived to initialize it, specifically at `pkg/k8s/secret.go:453` when accessing `timer.C`.

**Fix**: Implemented a nil-safe channel pattern where the timer channel is conditionally set based on whether the timer is initialized. A nil channel in a `select` statement blocks forever (never selected), eliminating the race condition between timer initialization and select evaluation.

---

## Technical Details

### Changes

**File**: `pkg/k8s/secret.go`
- Added nil-safe channel pattern before the select statement
- Changed `case <-timer.C:` to `case <-timerC:` where `timerC` is conditionally set
- When timer is nil, timerC remains nil and never gets selected
- When timer is initialized, timerC is set to timer.C and becomes active

### Impact

- ✅ **Eliminates startup panics** caused by the race condition
- ✅ **No functional changes** to secret synchronization behavior
- ✅ **No configuration changes** required
- ✅ **Fully backward compatible** with v1.1.0

---

## Upgrade Instructions

### Helm Chart Upgrade

```bash
helm upgrade push-to-k8s supporttools/push-to-k8s \
  --version 1.1.1 \
  --namespace push-to-k8s
```

### Docker Image

```bash
docker pull supporttools/push-to-k8s:v1.1.1
# or
docker pull supporttools/push-to-k8s:latest
```

### Direct Upgrade

If you're running v1.1.0 and experiencing startup panics, this upgrade is **highly recommended**. Simply update to v1.1.1 and redeploy.

---

## Who Should Upgrade?

**Priority**: HIGH

- ✅ **All v1.1.0 users** should upgrade to v1.1.1
- ✅ **Especially critical** if you've experienced startup panics or crashes
- ✅ **Recommended for all deployments** to prevent potential issues

---

## Changelog

### Fixed
- **Critical**: Fixed nil pointer dereference panic in `processDebouncedSecretQueue` that could occur on startup (pkg/k8s/secret.go:453)

### Changed
- Implemented nil-safe channel pattern for debounce timer in secret queue processor

---

## Known Issues

None at this time. This release specifically addresses the critical startup panic discovered in v1.1.0.

---

## Support

For issues, questions, or feature requests:
- **GitHub Issues**: https://github.com/supporttools/push-to-k8s/issues
- **Maintainer**: mattmattox (mmattox@support.tools)

---

## Full Changelog

See the [commit history](https://github.com/supporttools/push-to-k8s/compare/v1.1.0...v1.1.1) for complete details.
