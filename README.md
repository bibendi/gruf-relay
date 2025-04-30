# Gruf Relay - gRPC Proxy Server

Gruf Relay is a lean and performant gRPC proxy server crafted to optimize resource usage within microservice architectures, especially those leveraging single-threaded languages like Ruby. In Kubernetes deployments, a common pattern involves sidecar containers, which inevitably consume resources alongside your primary application. By deploying Gruf Relay within a pod, you unlock the ability to run multiple Ruby workers (hosting your Gruf application) as sub-processes within a single container. Gruf Relay then intelligently load balances gRPC requests across these workers, maximizing CPU utilization within the pod, without the overhead of additional sidecars or pod scaling.

Key features include built-in load balancing, which ensures requests are evenly distributed across available workers, and robust health checks that contribute to high availability. Kubernetes-native readiness and liveness probes are present for seamless integration with orchestration platforms. Comprehensive metrics provide detailed insights into performance and resource consumption. A built-in request queue is also present, preventing request drops during bursts of traffic, enhancing overall system reliability. Ultimately, Gruf Relay amplifies pod capacity by enabling multiple Ruby instances to coexist within a single pod, sharing sidecars and substantially reducing overall resource demand.

## Features

- **🔄 Random Load Balancing**: Distribute requests across healthy backend instances.
- **🏥 Health Checking**: Regular gRPC health checks with configurable intervals.
- **⚙️ Dynamic Configuration**: YAML config + environment variables support.
- **📊 Metrics Exposure**: Prometheus metrics endpoint for monitoring.
- **🔌 Worker Management**: Automated worker lifecycle management.
- **📈 Horizontal Scaling**: Easily scale backend worker instances.
- **📝 Structured Logging**: JSON-formatted logs with configurable levels.
- **✅ Request Handling**: Includes request queuing to help prevent request loss, offering enhanced reliability compared to a basic Gruf setup.

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
  proxy_timeout: "5s"
workers:
  count: 2
  start_port: 9000
  metrics_path: "/metrics"
  pool_size: 5
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
*   `SERVER_PROXY_TIMEOUT`: Timeout for proxy requests (default: `5s`). Must be a valid duration string (e.g., "10s", "1m", "1m30s").
*   `HEALTH_CHECK_INTERVAL`: Interval for health checks (default: `5s`).  Must be a valid duration string (e.g., "10s", "1m", "1m30s").
*   `HEALTH_CHECK_TIMEOUT`: Timeout for health checks (default: `3s`).  Must be a valid duration string (e.g., "10s", "1m", "1m30s").
*   `WORKERS_COUNT`: Number of backend workers (default: `2`).
*   `WORKERS_START_PORT`: Starting port for workers (default: `9000`).
*   `WORKERS_METRICS_PATH`: Path for worker metrics endpoint (default: `/metrics`).
*   `WORKERS_POOL_SIZE`: Size of the worker pool (default: `5`).
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
