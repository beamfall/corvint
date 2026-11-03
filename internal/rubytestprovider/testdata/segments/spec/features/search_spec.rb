# frozen_string_literal: true

require_relative '../spec_helper'

RSpec.shared_examples 'a catalog search' do
  before { visit '/warm' }

  it 'lists the matching item' do
    home = HomePage.new
    home.load
    home.search_for('widget')
    expect(ResultsPage.new.titles).to eq(['widget'])
  end
end

RSpec.describe 'search as a guest', type: :feature do
  include_examples 'a catalog search'
end

RSpec.describe 'search from the archive', type: :feature do
  include_examples 'a catalog search'
end

RSpec.describe 'search for a gadget', type: :feature do
  before { visit '/reset' }

  it 'lists the gadget' do
    home = HomePage.new
    home.load
    home.search_for('gadget')
    expect(ResultsPage.new.titles).to eq(['gadget'])
    expect(page).to have_css('.item', count: 1)
  end
end

RSpec.describe 'item page', type: :feature do
  it 'shows the gadget price' do
    item = ItemPage.new
    item.load(name: 'gadget')
    expect(item.price_text).to eq('450')
  end
end
