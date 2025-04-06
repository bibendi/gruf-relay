require_relative "../../../pkg/server/jobs_pb"
require_relative "../../../pkg/server/jobs_services_pb"

module Demo
  class JobsController < ::Gruf::Controllers::Base
    bind ::Demo::Jobs::Service

    ##
    # @return [Demo::GetJobResp] The job response
    #
    def get_job
      Demo::GetJobResp.new(id: 101)
    rescue StandardError => e
      fail!(:internal, :internal, "ERROR: #{e.message}")
    end
  end
end
