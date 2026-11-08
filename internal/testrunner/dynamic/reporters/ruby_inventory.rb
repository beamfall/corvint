# SPDX-License-Identifier: Apache-2.0
# Discovery only. Uses the frameworks' suite/list APIs; never runs test bodies.
# The output reuses the existing owned reporter document and Test.File member.
begin
require 'json'
output, runner, config = ARGV.shift(3)
raise 'missing selector delimiter' unless ARGV.shift == '--'
files = ARGV.dup
ARGV.clear
raise 'empty inventory scope' if files.empty?
case runner
when 'minitest', 'rails-test'
  require 'minitest'
when 'test-unit'
  require 'test/unit'
else
  raise 'unsupported runner'
end
require File.expand_path(config) unless config.empty?
files.each { |file| require File.expand_path(file) }
rows = []
add = lambda do |id, name, location|
  raise 'source location unavailable' unless location && location[0]
  root = File.realpath(Dir.pwd) + File::SEPARATOR
  file = File.realpath(location[0])
  raise 'source outside checkout' unless file.start_with?(root)
  rows << {ID: id, Name: name, File: file.delete_prefix(root), State: 'PASSED'}
  raise 'inventory cap exceeded' if rows.length > 4096
end
if runner == 'test-unit'
  visit = lambda do |node|
    if node.is_a?(Test::Unit::TestCase)
      add.call(node.name, node.method_name, node.class.instance_method(node.method_name).source_location)
    elsif node.is_a?(Test::Unit::TestSuite)
      node.tests.each { |child| visit.call(child) }
    else
      raise 'unsupported suite node'
    end
  end
  ObjectSpace.each_object(Class).select { |klass| klass < Test::Unit::TestCase }.each { |klass| visit.call(klass.suite) }
else
  Minitest::Runnable.runnables.each do |klass|
    klass.runnable_methods.each do |name|
      add.call("#{klass.name}##{name}", name, klass.instance_method(name).source_location)
    end
  end
end
raise 'duplicate identity' unless rows.map { |r| r[:ID] }.uniq.length == rows.length
profile = runner == 'test-unit' ? 'corvint-test-unit/0' : 'corvint-minitest/0'
File.open(output, File::WRONLY | File::CREAT | File::EXCL, 0600) do |f|
  f.write(JSON.generate(Profile: profile, Complete: true, Count: rows.length, Tests: rows.sort_by { |r| r[:ID] }, Problems: []))
end
# Framework autorun hooks must not turn discovery into execution.
exit! 0
rescue Exception => error
  warn "Ruby inventory incomplete: #{error.class}"
  exit! 1
end
