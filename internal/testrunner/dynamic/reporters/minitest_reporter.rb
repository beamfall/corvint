# SPDX-License-Identifier: Apache-2.0
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
    row = {ID: "#{result.klass}##{result.name}", Name: result.name, State: state}
    # TRE-V0-110: an exception Minitest attributes to one named test fails that
    # test with cause UNKNOWN; it never voids the other cases of the run.
    errors = result.failures.select { |f| f.is_a?(Minitest::UnexpectedError) }
    if state == 'FAILED' && !errors.empty?
      row[:FailureKind] = 'UNKNOWN'
      row[:Message] = errors.map { |f| "UnexpectedError: #{f.error.class.name}" }.uniq.join("\n")[0, 1024]
    end
    @rows << row
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
