# frozen_string_literal: true

require_relative '../lib/calc'

RSpec.describe Calc do
  it 'adds' do
    expect(Calc.new.add(1, 2)).to eq(3)
  end

  it 'differs downward' do
    expect(Calc.new.diff(3, 1)).to eq(2)
  end

  it 'differs upward' do
    expect(Calc.new.diff(1, 3)).to eq(2)
  end

  it 'fails' do
    expect(1).to eq(2)
  end
end
