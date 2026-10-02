# SPDX-License-Identifier: AGPL-3.0-or-later
require 'json'
require 'minitest'

output = ARGV.shift
if ARGV.first == '--require'
  ARGV.shift
  require File.expand_path(ARGV.shift)
end
raise 'missing selector delimiter' unless ARGV.shift == '--'
files = ARGV.dup
ARGV.clear

class CorvintReporter < Minitest::AbstractReporter
  attr_reader :rows
  def initialize(output)
    super()
    @output, @rows = output, []
  end
  def record(result)
    state = result.skipped? ? 'SKIPPED' : (result.passed? ? 'PASSED' : 'FAILED')
    state = 'UNKNOWN' if result.failures.any? { |f| f.is_a?(Minitest::UnexpectedError) }
    @rows << {ID: "#{result.klass}##{result.name}", Name: result.name, State: state}
  end
  def report
    File.open(@output, File::WRONLY | File::CREAT | File::EXCL, 0600) do |f|
      f.write(JSON.generate(Profile: 'corvint-minitest/0', Complete: true,
                            Count: @rows.length, Tests: @rows, Problems: []))
    end
  end
end

# Minitest's documented extension hook adds the reporter after the default aggregate exists.
Minitest.singleton_class.send(:define_method, :plugin_corvint_init) do |_options|
  Minitest.reporter << CorvintReporter.new(output)
end
Minitest.extensions << 'corvint'
files.each { |file| require File.expand_path(file) }
require 'minitest/autorun'
