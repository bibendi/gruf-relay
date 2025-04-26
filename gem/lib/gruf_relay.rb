# frozen_string_literal: true

require_relative "gruf_relay/version"

module GrufRelay
  class Error < StandardError; end
  class UnsupportedPlatformError < Error; end

  # Path to the binary for the current platform
  def self.binary_path
    @@binary_path ||= begin
      os = RbConfig::CONFIG["host_os"].downcase
      cpu = RbConfig::CONFIG["host_cpu"].downcase

      # Normalize OS
      os = case os
           when /linux/
             "linux"
           when /darwin|mac os/
             "darwin"
           else
             raise UnsupportedPlatformError, "Unsupported OS: #{os}"
           end

      # Normalize CPU architecture
      arch = case cpu
             when /x86_64|x64|amd64/
               "amd64"
             when /arm64|aarch64/
               "arm64"
             else
               raise UnsupportedPlatformError, "Unsupported architecture: #{cpu}"
             end

      binary_name = "gruf-relay-#{os}-#{arch}"

      # Path to the binary for this platform
      binary_path = File.expand_path("../exe/#{binary_name}", __dir__)

      unless File.exist?(binary_path)
        raise Error, "Missing binary for #{os}/#{arch}. Expected at #{binary_path}"
      end

      # Make sure the binary is executable
      File.chmod(0755, binary_path) unless File.executable?(binary_path)

      binary_path
    end
  end

  # Execute the gruf-relay binary with the given arguments
  def self.execute(*args)
    system(binary_path, *args)
  end
end
