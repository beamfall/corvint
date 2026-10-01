# SPDX-License-Identifier: AGPL-3.0-or-later
require 'json'
require 'test/unit'
require 'test/unit/ui/console/testrunner'

CORVINT_OUTPUT = ARGV.shift
files = ARGV.dup
ARGV.clear

class CorvintTestUnitRunner < Test::Unit::UI::Console::TestRunner
  def started(result)
    @corvint_rows = []
    @corvint_faults = {}
    super
  end
  def add_fault(fault)
    @corvint_faults[fault.test_name] = fault
    super
  end
  def test_finished(test)
    fault = @corvint_faults[test.name]
    state = case fault
            when Test::Unit::Failure then 'FAILED'
            when Test::Unit::Error then 'UNKNOWN'
            when Test::Unit::Omission, Test::Unit::Pending then 'SKIPPED'
            else 'PASSED'
            end
    @corvint_rows << {ID: test.name, Name: test.name, State: state}
    super
  end
  def finished(elapsed)
    super
    File.open(CORVINT_OUTPUT, File::WRONLY | File::CREAT | File::EXCL, 0600) do |f|
      f.write(JSON.generate(Profile: 'corvint-test-unit/0', Complete: true,
                            Count: @corvint_rows.length, Tests: @corvint_rows, Problems: []))
    end
  end
end
Test::Unit::AutoRunner.register_runner(:corvint) { CorvintTestUnitRunner }
files.each { |file| require File.expand_path(file) }
exit(Test::Unit::AutoRunner.run(false, nil, ['--runner=corvint']) ? 0 : 1)
