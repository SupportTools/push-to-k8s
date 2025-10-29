# Push-to-K8s Documentation

<div align="center">

<img src="press-kit/logos/logo-full-color.svg" alt="Push-to-K8s" width="400">

**Automatically synchronize Kubernetes secrets across all namespaces**

[![Version](https://img.shields.io/badge/version-1.0.0-326CE5?style=flat-square)](https://github.com/supporttools/push-to-k8s/releases)
[![License](https://img.shields.io/badge/license-MIT-326CE5?style=flat-square)](../LICENSE)
[![Kubernetes](https://img.shields.io/badge/kubernetes-1.19+-326CE5?style=flat-square)](https://kubernetes.io)

</div>

---

## Welcome

Push-to-K8s is a production-ready Kubernetes controller that automatically synchronizes labeled secrets from a source namespace to all other namespaces in your cluster. Simply label a secret once, and it propagates everywhere automatically.

## Quick Start

```bash
# 1. Install Push-to-K8s
kubectl apply -f https://raw.githubusercontent.com/supporttools/push-to-k8s/main/deploy.yaml

# 2. Label a secret
kubectl label secret my-secret push-to-k8s=source -n push-to-k8s

# 3. Watch it sync automatically to all namespaces!
kubectl get secrets my-secret --all-namespaces
```

👉 **[Full Quick Start Guide](getting-started/quick-start.md)**

---

## Documentation

### 🚀 Getting Started

Perfect for new users getting Push-to-K8s up and running.

- **[Installation](getting-started/installation.md)** - Deploy Push-to-K8s to your cluster
- **[Quick Start](getting-started/quick-start.md)** - Get syncing in 5 minutes
- **[Configuration](getting-started/configuration.md)** - Environment variables and settings

### 📖 User Guides

In-depth guides for common use cases and advanced features.

- **[Basic Usage](guides/basic-usage.md)** - Label secrets and verify synchronization
- **[Advanced Configuration](guides/advanced-configuration.md)** - Namespace exclusion, custom intervals
- **[Monitoring & Metrics](guides/monitoring.md)** - Prometheus metrics and observability
- **[Troubleshooting](guides/troubleshooting.md)** - Common issues and solutions
- **[Best Practices](guides/best-practices.md)** - Security and operational recommendations

### 🏗️ Architecture

Understand how Push-to-K8s works under the hood.

- **[Architecture Overview](architecture/overview.md)** - System design and components
- **[How It Works](architecture/how-it-works.md)** - Sync logic and namespace watching
- **[Design Decisions](architecture/design-decisions.md)** - Why things work the way they do

### 📊 API Reference

Technical reference for metrics and endpoints.

- **[Prometheus Metrics](api/metrics.md)** - Available metrics and queries
- **[Health Endpoints](api/health.md)** - Liveness and readiness checks
- **[Version Information](api/version.md)** - Build metadata

### 🤝 Contributing

Help improve Push-to-K8s!

- **[Development Guide](contributing/development.md)** - Build and run locally
- **[Testing](contributing/testing.md)** - Unit and integration tests
- **[Code of Conduct](contributing/code-of-conduct.md)** - Community guidelines
- **[Changelog](contributing/changelog.md)** - Release history

### 🎨 Press Kit

Branding and media resources for the community.

- **[Brand Assets](press-kit/README.md)** - Logos, colors, and usage guidelines
- **[Logos](press-kit/logos/)** - Download official logos
- **[Screenshots](press-kit/screenshots.md)** - Product screenshots and demos

---

## Use Cases

### Registry Credentials
Distribute Docker registry credentials across all namespaces so every pod can pull images:
```bash
kubectl create secret docker-registry regcred \
  --docker-server=registry.example.com \
  --docker-username=user \
  --docker-password=pass \
  -n push-to-k8s

kubectl label secret regcred push-to-k8s=source -n push-to-k8s
```

### TLS Certificates
Share TLS certificates cluster-wide for consistent HTTPS:
```bash
kubectl create secret tls wildcard-cert \
  --cert=cert.pem \
  --key=key.pem \
  -n push-to-k8s

kubectl label secret wildcard-cert push-to-k8s=source -n push-to-k8s
```

### API Keys & Tokens
Distribute API keys to all applications automatically:
```bash
kubectl create secret generic api-keys \
  --from-literal=stripe-key=sk_test_xxx \
  --from-literal=sendgrid-key=SG.xxx \
  -n push-to-k8s

kubectl label secret api-keys push-to-k8s=source -n push-to-k8s
```

---

## Features

- ✅ **Automatic Synchronization** - Label once, propagate everywhere
- ✅ **Namespace Watch** - New namespaces get secrets immediately
- ✅ **Selective Exclusion** - Skip specific namespaces with labels
- ✅ **Production Ready** - Battle-tested with comprehensive metrics
- ✅ **Zero Configuration** - Works out of the box with sensible defaults
- ✅ **Prometheus Metrics** - Built-in observability
- ✅ **Health Checks** - Liveness and readiness endpoints
- ✅ **Graceful Shutdown** - Proper cleanup on termination

---

## Requirements

- **Kubernetes**: 1.19 or higher
- **RBAC**: ClusterRole permissions (included in deploy.yaml)
- **Go**: 1.21+ (only for building from source)

---

## Community

- **GitHub**: [github.com/supporttools/push-to-k8s](https://github.com/supporttools/push-to-k8s)
- **Issues**: [Report bugs or request features](https://github.com/supporttools/push-to-k8s/issues)
- **Discussions**: [Ask questions and share ideas](https://github.com/supporttools/push-to-k8s/discussions)
- **Contact**: Matt Mattox (mmattox@support.tools)

---

## License

Push-to-K8s is open-source software licensed under the [MIT License](../LICENSE).

---

## Navigation

- [Home](index.md)
- [Getting Started](getting-started/installation.md)
- [User Guides](guides/basic-usage.md)
- [Architecture](architecture/overview.md)
- [API Reference](api/metrics.md)
- [Contributing](contributing/development.md)
- [Press Kit](press-kit/README.md)
