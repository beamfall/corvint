# frozen_string_literal: true

require_relative '../spec_helper'

RSpec.shared_examples 'a cart add' do
  it 'adds the widget' do
    expect(add_to_cart('widget')).to eq('widget: 300')
  end
end

RSpec.describe 'cart from search', type: :feature do
  include_examples 'a cart add'
end

RSpec.describe 'cart from an item page', type: :feature do
  include_examples 'a cart add'
end

RSpec.describe 'cart with a gadget', type: :feature do
  it 'adds the gadget and checks the total' do
    line = add_to_cart('gadget')
    expect(line).to start_with('gadget')
    expect(line).to end_with('450')
  end
end
