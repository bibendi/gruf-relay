# frozen_string_literal: true

require 'active_support/core_ext/string'

module Metrics
  module Gruf
    # Provides ActiveSupport::Notification instrumentation via interceptor
    class InstrumentationInterceptor < ::Gruf::Interceptors::ServerInterceptor
      EVENT_NAME = 'gruf_server.request'
      STATUS_CODES_MAP = ::GRPC::Core::StatusCodes.constants.each_with_object({}) do |status_name, hash|
        hash[::GRPC::Core::StatusCodes.const_get(status_name)] = status_name.to_s.downcase.camelize
      end.freeze

      def call
        return yield unless request.request_response?

        ActiveSupport::Notifications.instrument(EVENT_NAME, {request: request}) do |pl|
          pl[:status_name] = STATUS_CODES_MAP[::GRPC::Core::StatusCodes::OK]
          yield
        rescue ::GRPC::BadStatus => e
          pl[:exception_object] = e
          pl[:status_name] = STATUS_CODES_MAP[e.code]
          raise
        rescue => e
          pl[:exception_object] = e
          pl[:status_name] = 'InternalError'
          raise
        end
      end
    end
  end
end
