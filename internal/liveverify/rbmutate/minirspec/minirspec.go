// Package minirspec is a test double for RSpec: a small Ruby program that
// accepts the command line rbmutate passes (`--version`, or `--format json
// --out PATH --no-color SPEC`), runs `describe`/`it`/`expect(...).to eq(...)`
// examples, and writes RSpec's JSON summary shape. Tests use it to drive the
// Ruby mutation runner through a real Ruby interpreter on hosts without the
// rspec gem; it is never used outside tests.
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
until args.empty?
  arg = args.shift
  case arg
  when '--out' then out = args.shift
  when '--format' then args.shift
  when /\A--/ then nil
  else files << arg
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
def describe(name)
  $mini_stack.push(name.to_s)
  yield
  $mini_stack.pop
end
def it(name, &block)
  $mini_examples << [($mini_stack + [name]).join(' '), block]
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
  $mini_examples.each_with_index do |(name, block), index|
    status = 'passed'
    begin
      block.call
    rescue Exception
      status = 'failed'
    end
    results << { 'id' => "./#{files.first}[1:#{index + 1}]", 'full_description' => name, 'status' => status }
  end
end
failures = results.count { |result| result['status'] == 'failed' }
summary = { 'example_count' => results.size, 'failure_count' => failures, 'pending_count' => 0, 'errors_outside_of_examples_count' => errors }
File.write(out, JSON.generate('examples' => results, 'summary' => summary)) if out
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
