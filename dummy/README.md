# Gruf Relay - gRPC Proxy Server

Gruf Relay is a high-performance gRPC proxy server designed to optimize resource utilization in microservices environments, especially those leveraging single-core languages like Ruby. By load balancing gRPC requests across multiple backend pods, Gruf Relay allows you to effectively scale your applications within Kubernetes, maximizing CPU usage and reducing resource overhead compared to simply increasing pod counts. It features built-in load balancing and health checking capabilities to ensure optimal availability.

## Features

- **🔄 Random Load Balancing**: Distribute requests across healthy backend instances
- **🏥 Health Checking**: Regular gRPC health checks with configurable intervals
- **⚙️ Dynamic Configuration**: YAML config + environment variables support
- **📊 Metrics Exposure**: Prometheus metrics endpoint for monitoring
- **🔌 Process Management**: Automated worker process lifecycle management
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

### Config File (config.yaml)
```yaml
log_level: "debug"
host: "0.0.0.0"
port: 8080
health_check_interval: "5s"

workers:
  count: 3
  start_port: 9000
  metrics_path: "/metrics"

probes:
  enabled: true
  port: 5555

metrics:
  enabled: true
  metrics_port: 9394
  path: "/metrics"
```

### Environment Variables

The following environment variables can be used to override settings in the config.yaml file:

The following environment variables can be used to override settings in the config.yaml file:

LOG_LEVEL: Logging level (default: debug). Possible values: debug, info, warn, error.
LOG_FORMAT: Logging format (default: json). Possible values: json, text.
HOST: Host address for the gRPC proxy (default: 0.0.0.0).
PORT: Port for the gRPC proxy (default: 8080).
HEALTH_CHECK_INTERVAL: Interval for health checks (default: 5s).
WORKERS_COUNT: Number of backend worker processes (default: 2).
WORKERS_START_PORT: Starting port for worker processes (default: 9000).
WORKERS_METRICS_PATH: Path for worker metrics endpoint (default: /metrics).
PROBES_ENABLED: Enable/disable liveness/readiness probes (default: true).
PROBES_PORT: Port for liveness/readiness probes (default: 5555).
METRICS_ENABLED: Enable/disable metrics exposure (default: true).
METRICS_PORT: Port for Prometheus metrics (default: 9394).
METRICS_PATH: Path for Prometheus metrics (default: /metrics).

Example:
```bash
export PORT=8080
export WORKERS_COUNT=3
export METRICS_ENABLED=true
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
1. **Manager**: Controls worker processes lifecycle
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
dummy/script/job
```

#### Checking Health
```bash
dummy/script/health_check
```

## Contributing

Contributions welcome!

## License

MIT

---

Made with ❤️ by @bibendi
```
