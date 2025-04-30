# frozen_string_literal: true

module Metrics
  module Gruf
    class StatsCollector
      class Hook < ::Gruf::Hooks::Base
        def before_server_start(server:)
          StatsCollector.gruf_server = server
          Yabeda::Prometheus::Exporter.start_metrics_server!
        end
      end

      class << self
        attr_accessor :gruf_server

        delegate :call, to: :new
      end

      def call
        return unless server

        server.instance_variable_get(:@run_mutex).synchronize { measure_metrics }
      end

      private

      def measure_metrics
        return unless pool

        ::Yabeda.grpc_server_pool_ready_workers_total.set({}, pool.instance_variable_get(:@ready_workers)&.size)
        ::Yabeda.grpc_server_pool_workers_total.set({}, pool.instance_variable_get(:@workers)&.size)
        ::Yabeda.grpc_server_pool_initial_size.set({}, server.instance_variable_get(:@pool_size)&.to_i)
        ::Yabeda.grpc_server_poll_period.set({}, server.instance_variable_get(:@poll_period)&.to_i)
      end

      def pool
        @pool ||= server&.instance_variable_get(:@pool)
      end

      def server
        @server ||= self.class.gruf_server&.server
      end
    end
  end
end
