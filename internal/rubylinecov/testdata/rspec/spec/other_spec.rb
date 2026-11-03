# frozen_string_literal: true

require_relative '../lib/calc'

RSpec.describe 'other' do
  it 'adds again' do
    expect(Calc.new.add(2, 2)).to eq(4)
  end
end
