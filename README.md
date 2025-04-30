# Gruf Relay - gRPC Proxy Server

Gruf Relay is a high-performance gRPC proxy server designed to optimize resource utilization in microservices environments, especially those leveraging single-core languages like Ruby. By load balancing gRPC requests across multiple backend pods, Gruf Relay allows you to effectively scale your applications within Kubernetes, maximizing CPU usage and reducing resource overhead compared to simply increasing pod counts. It features built-in load balancing and health checking capabilities to ensure optimal availability.

## Features

- **🔄 Random Load Balancing**: Distribute requests across healthy backend instances
- **🏥 Health Checking**: Regular gRPC health checks with configurable intervals
- **⚙️ Dynamic Configuration**: YAML config + environment variables support
- **📊 Metrics Exposure**: Prometheus metrics endpoint for monitoring
- **🔌 Worker Management**: Automated worker lifecycle management
- **📈 Horizontal Scaling**: Easily scale backend worker instances
- **📝 Structured Logging**: JSON-formatted logs with configurable levels

## Installation

### Quick Install
```bash
# Install with Go
go install github.com/bibendi/gruf-relay/cmd/gruf-relay@latest

# Or build from source
git clone https://github.com/bibendi/gruf-relay
cd gruf-relay
make build
```

## Configuration

Gruf Relay prioritizes configuration in the following order:

1.  **Environment Variables**: Override configuration settings.
2.  **Configuration File**: If the `CONFIG_PATH` environment variable is set, Gruf Relay attempts to load the configuration from the specified YAML file. If not set, it defaults to searching for a file named `gruf-relay.yml` in the current directory.
3.  **Default Values**: If a configuration value is not provided by either environment variables or the config file, the default value is used.

### Config File (gruf-relay.yml)
```yaml
log:
  level: "debug"
  format: "json"
server:
host: "0.0.0.0"
port: 8080
workers:
  count: 2
  start_port: 9000
  metrics_path: "/metrics"
health_check:
  interval: "5s"
  timeout: "3s"
probes:
  enabled: true
  port: 5555
metrics:
  enabled: true
  port: 9394
  path: "/metrics"
  interval: "5s"
```

### Environment Variables

The following environment variables can be used to override settings in the `config.yaml` file. Environment variables take precedence over the configuration file.

*   `LOG_LEVEL`: Logging level (default: `debug`). Possible values: `debug`, `info`, `warn`, `error`.
*   `LOG_FORMAT`: Logging format (default: `json`). Possible values: `json`, `text`.
*   `SERVER_HOST`: Host address for the gRPC proxy (default: `0.0.0.0`).
*   `SERVER_PORT`: Port for the gRPC proxy (default: `8080`).
*   `HEALTH_CHECK_INTERVAL`: Interval for health checks (default: `5s`).  Must be a valid duration string (e.g., "10s", "1m", "1m30s").
*   `HEALTH_CHECK_TIMEOUT`: Timeout for health checks (default: `3s`).  Must be a valid duration string (e.g., "10s", "1m", "1m30s").
*   `WORKERS_COUNT`: Number of backend workers (default: `2`).
*   `WORKERS_START_PORT`: Starting port for workers (default: `9000`).
*   `WORKERS_METRICS_PATH`: Path for worker metrics endpoint (default: `/metrics`).
*   `PROBES_ENABLED`: Enable/disable liveness/readiness probes (default: `true`).
*   `PROBES_PORT`: Port for liveness/readiness probes (default: `5555`).
*   `METRICS_ENABLED`: Enable/disable metrics exposure (default: `true`).
*   `METRICS_PORT`: Port for Prometheus metrics (default: `9394`).
*   `METRICS_PATH`: Path for Prometheus metrics (default: `/metrics`).
*   `METRICS_INTERVAL`: Interval for metrics collection (default: `5s`). Must be a valid duration string (e.g., "10s", "1m", "1m30s").

Example:

```bash
export SERVER_PORT=8081
export WORKERS_COUNT=3
export METRICS_ENABLED=true
export HEALTH_CHECK_INTERVAL=10s
```

## Usage

```bash
# Start server with config
./gruf-relay
```

### Endpoints

| Endpoint          | Port  | Description                                  |
|-------------------|-------|----------------------------------------------|
| gRPC Proxy        | 8080  | Main proxy endpoint                           |
| Metrics           | 9394  | Prometheus metrics                            |
| Liveness Probe    | 5555  | Kubernetes liveness check (`/liveness`)       |
| Readiness Probe   | 5555  | Kubernetes readiness check (`/readiness`)     |
| Startup Probe     | 5555  | Kubernetes startup check (`/startup`)         |

## Architecture

### Key Components
1. **Manager**: Controls worker lifecycle
2. **Health Checker**: Monitors worker availability
3. **Random Balancer**: Distributes requests evenly
4. **Metrics Server**: Exposes Prometheus metrics
5. **Probes Server**: Provides endpoints for liveness, readiness and startup probes.

## Example Usage

### Probes

#### Startup Probe
```bash
curl -v http://localhost:5555/startup
```

#### Readiness Probe
```bash
curl -v http://localhost:5555/readiness
```

#### Liveness Probe
```bash
curl -v http://localhost:5555/liveness
```

### Scripts

#### Triggering a Job
```bash
example/script/job
```

#### Checking Health
```bash
example/script/health_check
```

## Contributing

Contributions welcome!

## License

MIT

---

Made with ❤️ by @bibendi
```
