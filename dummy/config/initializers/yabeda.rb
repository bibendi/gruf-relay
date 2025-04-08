# frozen_string_literal: true

::Yabeda.configure do
  gauge :grpc_server_pool_ready_workers_total, comment: 'Number of non-busy workers in thread pool'
  gauge :grpc_server_pool_workers_total, comment: 'Total number of workers in thread pool'
  gauge :grpc_server_pool_initial_size, comment: 'Initial size of thread pool'
  gauge :grpc_server_poll_period, comment: 'Polling period for thread pool'

  counter :grpc_server_requests_total,
    tags: %i[grpc_type grpc_client_type grpc_client_method grpc_client_service
      grpc_client_code],
    comment: 'A counter of the total number of gRPC requests processed'

  histogram :grpc_server_request_duration,
    unit: :seconds,
    tags: %i[grpc_type grpc_client_type grpc_client_method grpc_client_service
      grpc_client_code],
    buckets: [0.001, 0.005, 0.01, 0.02, 0.04, 0.1, 0.2, 0.5, 0.8, 1, 1.5, 2, 5, 15, 30, 60],
    comment: 'Histogram of GRPC server request duration'

  collect do
    Metrics::Gruf::StatsCollector.call
  end
end
