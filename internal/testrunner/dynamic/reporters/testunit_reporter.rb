# SPDX-License-Identifier: Apache-2.0
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
            when Test::Unit::Omission, Test::Unit::Pending then 'SKIPPED'
            else 'PASSED'
            end
    row = {ID: test.name, Name: test.name, State: state}
    # TRE-V0-115: an exception raised in a named test fails that test with
    # cause UNKNOWN, naming only the exception class.
    if fault.is_a?(Test::Unit::Error)
      row[:State] = 'FAILED'
      row[:FailureKind] = 'UNKNOWN'
      row[:Message] = "Error: #{fault.exception.class.name}"[0, 1024]
    end
    @corvint_rows << row
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
