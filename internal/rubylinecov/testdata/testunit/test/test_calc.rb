# frozen_string_literal: true

require 'test/unit'
require_relative '../lib/calc'

class TestCalc < Test::Unit::TestCase
  def test_add
    assert_equal 3, Calc.new.add(1, 2)
  end

  def test_diff_down
    assert_equal 2, Calc.new.diff(3, 1)
  end
end
