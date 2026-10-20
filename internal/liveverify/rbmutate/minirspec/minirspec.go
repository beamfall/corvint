// Package minirspec is a test double for RSpec: a small Ruby program that
// accepts the command line rbmutate passes (`--version`, or `--options FILE
// --format json --out PATH --no-color SPEC[:LINE...]`), runs `describe`/`it`/`expect(...).to eq(...)`
// examples, and writes RSpec's JSON summary shape. Tests use it to drive the
// Ruby mutation runner through a real Ruby interpreter on hosts without the
// rspec gem; it is never used outside tests. As in RSpec, `SPEC:L1:L2` runs
// only the examples whose own line, or an enclosing group's line, is listed.
package minirspec

import (
	"os"
	"path/filepath"
)

// Source is the program. A load-time exception (a mutant that no longer
// parses, a missing constant) is an error outside examples, as in RSpec.
const Source = `#!/usr/bin/env ruby
require 'json'
args = ARGV.dup
if args.include?('--version')
  puts 'minirspec 0'
  exit 0
end
out = nil
files = []
selected = nil
until args.empty?
  arg = args.shift
  case arg
  when '--out' then out = args.shift
  when '--format', '--options' then args.shift
  when /\A--/ then nil
  else
    if (match = arg.match(/\A(.+?\.rb)((?::\d+)+)\z/))
      files << match[1]
      selected = (selected || []) + match[2].split(':').reject(&:empty?).map(&:to_i)
    else
      files << arg
    end
  end
end
class MiniFailure < StandardError; end
class MiniExpectation
  def initialize(actual)
    @actual = actual
  end

  def to(matcher)
    raise MiniFailure, "expected #{matcher[1].inspect}, got #{@actual.inspect}" unless matcher[0].call(@actual)
  end
end
$mini_examples = []
$mini_stack = []
$mini_lines = []
def describe(name)
  $mini_stack.push(name.to_s)
  $mini_lines.push(caller_locations(1, 1).first.lineno)
  yield
  $mini_lines.pop
  $mini_stack.pop
end
def it(name, &block)
  $mini_examples << [($mini_stack + [name]).join(' '), block, $mini_lines + [caller_locations(1, 1).first.lineno]]
end
def expect(actual)
  MiniExpectation.new(actual)
end
def eq(expected)
  [->(actual) { actual == expected }, expected]
end
errors = 0
begin
  files.each { |file| load File.expand_path(file) }
rescue Exception => e
  warn "error outside examples: #{e.class}: #{e.message}"
  errors = 1
end
results = []
if errors.zero?
  $mini_examples.each_with_index do |(name, block, lines), index|
    next if selected && (lines & selected).empty?
    result = { 'id' => "./#{files.first}[1:#{index + 1}]", 'full_description' => name, 'status' => 'passed' }
    begin
      block.call
    rescue Exception => e
      result['status'] = 'failed'
      result['exception'] = { 'class' => e.class.name, 'message' => e.message }
    end
    results << result
  end
end
failures = results.count { |result| result['status'] == 'failed' }
summary = { 'example_count' => results.size, 'failure_count' => failures, 'pending_count' => 0, 'errors_outside_of_examples_count' => errors }
File.write(out, JSON.generate('version' => '3.13.0', 'examples' => results, 'summary' => summary)) if out
exit(errors > 0 || failures > 0 ? 1 : 0)
`

// Install writes the program as an executable named rspec in dir and
// returns its path.
func Install(dir string) (string, error) {
	target := filepath.Join(dir, "rspec")
	if err := os.WriteFile(target, []byte(Source), 0o755); err != nil {
		return "", err
	}
	return target, nil
}
