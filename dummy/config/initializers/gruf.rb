# frozen_string_literal: true

require_relative "../../pkg/server/jobs_pb"
require_relative "../../pkg/server/jobs_services_pb"
require_relative "../../lib/metrics/gruf/stats_collector"
require_relative "../../lib/metrics/gruf/metrics_subscriber"
require_relative "../../lib/metrics/gruf/instrumentation_interceptor"

patch_enabled = !ENV["GRUF_BACKLOG_PATCH"].to_s.empty?

module Patches
  module Backlog
    module Pool
      def schedule(*args, &blk)
        return if blk.nil?
        @stop_mutex.synchronize do
          if @stopped
            GRPC.logger.warn("did not schedule job, already stopped")
            return
          end
          GRPC.logger.info("schedule another job")
          @jobs << [blk, args]
        end
      end

      def start
        @stop_mutex.synchronize do
          fail "already stopped" if @stopped
        end
        until @workers.size == @size.to_i
          next_thread = Thread.new do
            catch(:exit) do  # allows { throw :exit } to kill a thread
              loop_execute_jobs
            end
            remove_current_thread
          end
          @workers << next_thread
        end
      end

      def stop
        GRPC.logger.info("stopping, will wait for all the workers to exit")
        @stop_mutex.synchronize do  # wait @keep_alive seconds for workers to stop
          @stopped = true
          @workers.size.times { @jobs << [proc { throw :exit }, []] }
          @stop_cond.wait(@stop_mutex, @keep_alive) if @workers.size > 0
        end
        forcibly_stop_workers
        GRPC.logger.info("stopped, all workers are shutdown")
      end

      protected

      def loop_execute_jobs
        loop do
          blk, args = @jobs.pop
          blk.call(*args)
        rescue StandardError, GRPC::Core::CallError => e
          GRPC.logger.warn("Error in worker thread")
          GRPC.logger.warn(e)
        end
      end
    end

    module RpcServer
      def available?(an_rpc)
        jobs_count, max = @pool.jobs_waiting, @max_waiting_requests
        GRPC.logger.info("waiting: #{jobs_count}, max: #{max}")
        return an_rpc if @pool.jobs_waiting <= @max_waiting_requests
        GRPC.logger.warn("NOT AVAILABLE: too many jobs_waiting: #{an_rpc}")
        noop = proc { |x| x }

        # Create a new active call that knows that metadata hasn"t been
        # sent yet
        c = ActiveCall.new(an_rpc.call, noop, noop, an_rpc.deadline, metadata_received: true, started: false)
        c.send_status(GRPC::Core::StatusCodes::RESOURCE_EXHAUSTED, "No free threads in thread pool")
        nil
      end
    end
  end
end

if patch_enabled
  ::GRPC::Pool.prepend(Patches::Backlog::Pool)
  ::GRPC::RpcServer.prepend(Patches::Backlog::RpcServer)
  Rails.logger.info("Gruf backlog patch enabled")
end

Gruf.configure do |c|
  c.logger = Rails.logger
  c.grpc_logger = Rails.logger
  c.server_binding_url = "0.0.0.0:8080"
  c.rpc_server_options = c.rpc_server_options.merge(
    pool_size: ENV.fetch("RAILS_MAX_THREADS", "5").to_i,
    pool_keep_alive: 1, # sec
    max_waiting_requests: 20
  )
  c.interceptors.use(
    Gruf::Interceptors::Instrumentation::RequestLogging::Interceptor,
    log_parameters: true,
    # ignore_methods: ["grpc.health.v1.health.check"]
  )
  c.interceptors.use(Metrics::Gruf::InstrumentationInterceptor)
  c.hooks.use(Metrics::Gruf::StatsCollector::Hook)
  c.health_check_enabled = true
  c.event_listener_proc = ->(event) do
    c.logger.error("Gruf rejected request with: #{event}")
  end
  c.backtrace_on_error = true
  c.use_exception_message = true
end

Metrics::Gruf::MetricsSubscriber.subscribe(
  metrics: true
)

# raise "Chaos testing!" if rand(2) == 0
