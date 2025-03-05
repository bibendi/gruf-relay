# frozen_string_literal: true

require_relative "../../pkg/server/jobs_pb"
require_relative "../../pkg/server/jobs_services_pb"

Gruf.configure do |c|
  c.logger = Logger.new(STDOUT)
  c.rpc_server_options = c.rpc_server_options.merge(pool_size: 5)
  c.interceptors.use(
    Gruf::Interceptors::Instrumentation::RequestLogging::Interceptor,
    log_parameters: true
  )
  c.health_check_enabled = true
end
