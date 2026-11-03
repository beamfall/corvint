# frozen_string_literal: true

require_relative '../../app/shop'

RSpec.shared_examples 'a widget price' do
  it 'prices a widget' do
    expect(Shop::Pricing.price_for('widget')).to eq(300)
  end
end

RSpec.describe 'pricing for a guest' do
  include_examples 'a widget price'
end

RSpec.describe 'pricing for a member' do
  include_examples 'a widget price'
end

RSpec.describe 'pricing for a gadget', corvint_level: 'integration' do
  it 'prices a gadget' do
    expect(Shop::Pricing.price_for('gadget')).to eq(450)
  end
end
