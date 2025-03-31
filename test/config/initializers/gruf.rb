# frozen_string_literal: true

logger = Logger.new($stdout)
logger.level = Logger::INFO

require_relative "../../pkg/server/jobs_pb"
require_relative "../../pkg/server/jobs_services_pb"

Gruf.configure do |c|
  c.logger = logger
  c.grpc_logger = logger
  c.rpc_server_options = c.rpc_server_options.merge(pool_size: 5)
  c.interceptors.use(
    Gruf::Interceptors::Instrumentation::RequestLogging::Interceptor,
    log_parameters: true,
    # ignore_methods: ["grpc.health.v1.health.check"]
  )
  c.health_check_enabled = true
  c.event_listener_proc = ->(event) do
    c.logger.error("Gruf rejected request with: #{event}")
  end
end
