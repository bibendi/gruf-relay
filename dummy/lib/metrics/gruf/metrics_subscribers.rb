# frozen_string_literal: true
module Metrics
  module Gruf
    # Emits yabeda metrics by subscribing to InstrumentationInterceptor notifications
    module MetricsSubscriber
      class << self
        # @param [Hash] opts
        # @option opts [Boolean] metrics (true) enables or disables yabeda metrics
        def subscribe(opts = {})
          # rubocop:disable Metrics/AbcSize, Metrics/MethodLength, Metrics/CyclomaticComplexity, Metrics/PerceivedComplexity
          ActiveSupport::Notifications.subscribe('gruf_server.request') do |*args|
            event = ActiveSupport::Notifications::Event.new(*args)
            request = event.payload[:request]
            grpc_service = request.service_key
            grpc_method = request.method_key.to_s

            if opts.fetch(:metrics, true)
              emit_request_metric(grpc_method, grpc_service, event.duration, event.payload[:status_name])
            end
          end
        end

        private

        def emit_request_metric(grpc_method, grpc_service, duration, status)
          request_tags = {
            grpc_type: :unary,
            grpc_client_type: :unary,
            grpc_client_method: grpc_method,
            grpc_client_service: grpc_service,
            grpc_client_code: status
          }
          Yabeda.grpc_server_requests_total.increment(request_tags)
          Yabeda.grpc_server_request_duration.measure(request_tags, duration.fdiv(1000))
        end
      end
    end
  end
end
