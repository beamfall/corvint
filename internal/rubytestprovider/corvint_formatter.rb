# frozen_string_literal: true

# Corvint-shipped RSpec JSON formatter (LPCV-V0-136, LPCV-V0-138). The Ruby
# provider writes this file under .corvint/rspec for one run and removes it
# afterwards; it is never loaded by the project otherwise.
#
# It extends RSpec's stock JSON report with two members only:
#   - shared_group_inclusion_backtrace on every example: one
#     {"inclusion_location": "<file>:<line>"} frame per shared group the
#     example was included through, [] when it was not included through one;
#   - corvint_runtime.container: the 64-hex ID of the container the RSpec
#     process ran in, read from the /etc/hostname bind mount docker creates,
#     and absent when that cannot be read.
require 'rspec/core/formatters/json_formatter'

class CorvintRSpecJsonFormatter < RSpec::Core::Formatters::JsonFormatter
  RSpec::Core::Formatters.register self, :message, :dump_summary, :dump_profile, :stop, :seed, :close

  def close(notification)
    runtime = {}
    container = corvint_container
    runtime[:container] = container if container
    @output_hash[:corvint_runtime] = runtime
    super
  end

  private

  def format_example(example)
    frames = example.metadata[:shared_group_inclusion_backtrace] || []
    super.merge(shared_group_inclusion_backtrace: frames.map { |frame| { inclusion_location: frame.formatted_inclusion_location } })
  end

  def corvint_container
    File.foreach('/proc/self/mountinfo') do |line|
      fields = line.split(' ')
      next unless fields[4] == '/etc/hostname'

      match = %r{/containers/([0-9a-f]{64})/hostname\z}.match(fields[3])
      return match[1] if match
    end
    nil
  rescue SystemCallError, IOError
    nil
  end
end
