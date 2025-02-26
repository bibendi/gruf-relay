#!/usr/bin/env ruby
# frozen_string_literal: true

lib_path = File.expand_path("../lib", __dir__)
$LOAD_PATH.unshift(lib_path) unless $LOAD_PATH.include?(lib_path)

Signal.trap("INT") do
  puts "Shutdown #{Process.pid}"
  exit 0
end

Signal.trap("TERM") do
  puts "Shutdown #{Process.pid}"
  exit 0
end

loop do
  puts "Working hard #{Process.pid}"
  sleep 1
end
