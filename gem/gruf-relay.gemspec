# frozen_string_literal: true

require_relative "lib/gruf_relay/version"

# Get the platform from environment variable
platform = ENV["PLATFORM"] || "ruby"
gem_version = ENV["GEM_VERSION"] || GrufRelay::VERSION
binary_name = ENV["BINARY_NAME"] || "gruf-relay"

Gem::Specification.new do |spec|
  spec.name = "gruf-relay"
  spec.version = gem_version
  spec.authors = ["Misha Merkushin aka bibendi"]
  spec.email = ["merkushin.m.s@gmail.com"]
  spec.platform = platform unless platform == "ruby"

  spec.summary = "gRPC Gruf Relay service"
  spec.description = "A Go-based gRPC Gruf Relay service wrapped in a Ruby gem"
  spec.homepage = "https://github.com/bibendi/gruf-relay"
  spec.license = "MIT"
  spec.required_ruby_version = ">= 2.7.0"

  spec.metadata["homepage_uri"] = spec.homepage
  spec.metadata["source_code_uri"] = spec.homepage + "/tree/master"
  spec.metadata["changelog_uri"] = spec.homepage + "/blob/master/CHANGELOG.md"

  # Include only the binary for the current platform
  spec.bindir = "bin"
  spec.executables = ["gruf-relay"]

  # Include essential files
  spec.files = [
    "exe/#{binary_name}",
    "bin/gruf-relay",
    "lib/gruf_relay.rb",
    "lib/gruf_relay/version.rb"
  ]

  spec.require_paths = ["lib"]
end
