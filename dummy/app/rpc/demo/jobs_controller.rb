require_relative "../../../pkg/server/jobs_pb"
require_relative "../../../pkg/server/jobs_services_pb"

module Demo
  class JobsController < ::Gruf::Controllers::Base
    bind ::Demo::Jobs::Service

    ##
    # @return [Demo::GetJobResp] The job response
    #
    def get_job
      calculate_fibonacci(20) # simulate CPU
      sleep 0.03 # simulate IO
      calculate_fibonacci(20) # simulate CPU
      sleep 0.03 # simulate IO

      Demo::GetJobResp.new(id: 101)
    rescue StandardError => e
      fail!(:internal, :internal, "ERROR: #{e.message}")
    end

    private

    def calculate_fibonacci(n)
      if n <= 1
        n
      else
        calculate_fibonacci(n - 1) + calculate_fibonacci(n - 2)
      end
    end
  end
end
