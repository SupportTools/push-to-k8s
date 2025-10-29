<div align="center">

<img src="docs/press-kit/logos/logo-full-color.svg" alt="Push-to-K8s Logo" width="400">

# Push-to-K8s

**Automatically synchronize Kubernetes secrets across all namespaces**

[![Version](https://img.shields.io/badge/version-1.0.0-326CE5?style=flat-square)](https://github.com/supporttools/push-to-k8s/releases)
[![Build](https://img.shields.io/badge/build-passing-00C853?style=flat-square)](https://github.com/supporttools/push-to-k8s/actions)
[![License](https://img.shields.io/badge/license-MIT-326CE5?style=flat-square)](LICENSE)
[![Kubernetes](https://img.shields.io/badge/kubernetes-1.19+-326CE5?style=flat-square)](https://kubernetes.io)

---

</div>

Push-to-K8s is a Kubernetes controller that automatically synchronizes secrets from a source namespace to all other namespaces in your cluster. It watches for new namespaces and keeps secrets in sync across your entire cluster with **real-time change detection**.

## 🎯 Label once, propagate everywhere

## ✨ What's New in v1.1.0

🚀 **Real-Time Secret Synchronization**: Changes to source secrets now propagate to all namespaces in **5-10 seconds** instead of waiting up to 15 minutes!

- ⚡ Event-based sync with debounced batching
- 🔄 Automatic deletion propagation
- 🛡️ Built-in rate limiting to protect your API server
- 📊 Fully configurable with sensible defaults

[Quick Start Guide](docs/QUICK_START_SECRET_WATCHER.md) | [Technical Details](docs/SECRET_WATCHER_IMPLEMENTATION.md)

## Features

- **Real-Time Synchronization**: Source secret changes propagate in 5-10 seconds
- **Automatic Synchronization**: Syncs labeled secrets from a source namespace to all other namespaces
- **Namespace Watch**: Automatically syncs secrets to newly created namespaces in real-time
- **Deletion Propagation**: Removed source secrets are cleaned up from all target namespaces
- **Debounced Batching**: Rapid changes are batched together to reduce API load
- **Rate Limiting**: Configurable rate limiting prevents API server overload
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

```bash
# 1. Install Push-to-K8s
kubectl apply -f https://raw.githubusercontent.com/supporttools/push-to-k8s/main/deploy.yaml

# 2. Create and label a secret
kubectl create secret generic my-secret \
  --from-literal=key=value \
  -n push-to-k8s

kubectl label secret my-secret push-to-k8s=source -n push-to-k8s

# 3. Verify it synced to all namespaces
kubectl get secrets my-secret --all-namespaces
```

📖 **[Full Quick Start Guide →](docs/getting-started/quick-start.md)**

## Documentation

📚 **[Complete Documentation →](docs/index.md)**

**Getting Started:**
- [Installation](docs/getting-started/installation.md)
- [Quick Start](docs/getting-started/quick-start.md)
- [Configuration](docs/getting-started/configuration.md)

**Guides:**
- [Basic Usage](docs/guides/basic-usage.md)
- [Monitoring & Metrics](docs/guides/monitoring.md)
- [Troubleshooting](docs/guides/troubleshooting.md)

**More:**
- [Architecture](docs/architecture/overview.md)
- [Contributing](docs/contributing/development.md)
- [Press Kit](docs/press-kit/README.md)

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `NAMESPACE` | Source namespace for secrets | `push-to-k8s` |
| `DEBUG` | Enable debug logging | `false` |
| `METRICS_PORT` | Metrics server port | `9090` |
| `SYNC_INTERVAL` | Sync interval (minutes) | `15` |
| `EXCLUDE_NAMESPACE_LABEL` | Namespace exclusion label | `""` |

📖 **[Full Configuration Reference →](docs/getting-started/configuration.md)**


## How It Works

1. **Label a secret** with `push-to-k8s=source` in the source namespace
2. **Controller detects** the labeled secret
3. **Syncs automatically** to all namespaces (excluding those with exclusion labels)
4. **Watches for new namespaces** and syncs immediately
5. **Periodic sync** checks for updates every 15 minutes (configurable)

📖 **[Architecture Details →](docs/architecture/how-it-works.md)**


## Monitoring

Push-to-K8s exposes Prometheus metrics on port 9090:

- `/metrics` - Prometheus metrics
- `/healthz` - Health check
- `/version` - Version information

📖 **[Monitoring Guide →](docs/guides/monitoring.md)** | **[API Reference →](docs/api/metrics.md)**

## Troubleshooting

**Common issues:**
- Pod not starting → Check RBAC permissions
- Secrets not syncing → Verify label `push-to-k8s=source`
- Namespace excluded → Check for exclusion labels

📖 **[Full Troubleshooting Guide →](docs/guides/troubleshooting.md)**

## Security

- **RBAC**: Controller needs ClusterRole permissions for namespaces and secrets
- **Best Practices**: Only label secrets that should be cluster-wide
- **Exclusions**: Use namespace labels to exclude sensitive namespaces

📖 **[Security Best Practices →](docs/guides/best-practices.md)**


## Contributing

We welcome contributions! Whether it's bug reports, feature requests, documentation, or code.

📖 **[Development Guide →](docs/contributing/development.md)**

```bash
# Quick start for contributors
git clone https://github.com/supporttools/push-to-k8s.git
cd push-to-k8s
go test ./...
go build -o push-to-k8s main.go
```

## Community & Support

- 🐛 **[Report Issues](https://github.com/supporttools/push-to-k8s/issues)** - Bug reports and feature requests
- 💬 **[Discussions](https://github.com/supporttools/push-to-k8s/discussions)** - Questions and community chat
- 📖 **[Documentation](docs/index.md)** - Comprehensive guides
- 🎨 **[Press Kit](docs/press-kit/README.md)** - Logos and brand assets

## License

Push-to-K8s is open-source software licensed under the [MIT License](LICENSE).

---

<div align="center">

**[Documentation](docs/index.md)** • **[Quick Start](docs/getting-started/quick-start.md)** • **[Contributing](docs/contributing/development.md)** • **[Press Kit](docs/press-kit/README.md)**

Made with ❤️ by the Push-to-K8s community

</div>
