# Push-to-K8s

Push-to-K8s is a tool that syncs secrets from a source namespace to all other namespaces in a Kubernetes cluster. It periodically checks for changes and keeps the secrets in sync across all namespaces.

## Installation

To use this tool, you can clone the repository and build the binary using the provided `main.go` file and the relevant packages in the `pkg` directory.

```bash
git clone https://github.com/supporttools/push-to-k8s.git
cd push-to-k8s
go build main.go
```

## Usage

You can run the built binary to start the synchronization process. The tool reads its configuration from environment variables. Make sure to set the required environment variables before running the tool.

```bash
./main
```

The tool performs the following actions:
- Loads configuration from environment variables.
- Initializes a Kubernetes client.
- Starts a Prometheus metrics server for monitoring.
- Initiates periodic secret synchronization.
- Watches for namespace changes and syncs secrets as needed.

## Configuration

The tool expects the following environment variables for configuration:
- `DEBUG`: Set to `true` for debug mode.
- `METRICS_PORT`: Port for Prometheus metrics server.
- `NAMESPACE`: Source namespace for secrets.
- `EXCLUDE_NAMESPACE_LABEL`: Label for excluding certain namespaces.
- `SYNC_INTERVAL`: Interval in minutes for periodic sync.

## Packages

The project is structured with the following packages:
- `config`: Handles loading configuration from environment variables.
- `logging`: Manages logging setup and configuration.
- `metrics`: Provides functions for Prometheus metrics collection.
- `k8s`: Contains logic for Kubernetes client initialization, secret syncing, and namespace watching.
- `version`: Package with version information set during build time.

## Contributing

Contributions to this project are welcome. You can submit bug reports, feature requests, or pull requests through GitHub.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.