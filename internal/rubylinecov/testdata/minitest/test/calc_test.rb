# frozen_string_literal: true

require 'minitest/autorun'
require 'rbconfig'
require_relative '../lib/calc'

class CalcTest < Minitest::Test
  def test_add
    assert_equal 3, Calc.new.add(1, 2)
  end

  def test_diff_down
    assert_equal 2, Calc.new.diff(3, 1)
  end

  def test_diff_up
    assert_equal 2, Calc.new.diff(1, 3)
  end

  def test_tool
    assert system(RbConfig.ruby, 'bin/tool')
  end

  def test_fails
    assert_equal 1, 2
  end
end
