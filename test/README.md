# README

## Usage

Run Gruf server:
```sh
bundle exec gruf --host 0.0.0.0:9000 --health-check --backtrace-on-error
```

Check server health
```sh
grpcurl -plaintext --proto config/health.proto 0.0.0.0:9000 grpc.health.v1.Health/Check
```
