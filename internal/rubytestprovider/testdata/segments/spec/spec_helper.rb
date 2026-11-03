# frozen_string_literal: true

require 'capybara/rspec'
require 'site_prism'
require_relative '../app/shop'
require_relative 'pages/home_page'
require_relative 'pages/results_page'
require_relative 'pages/item_page'
require_relative 'support/steps/shop_steps'

Capybara.app = Shop::App.new

RSpec.configure do |config|
  config.include ShopSteps, type: :feature
  config.before(:each, type: :feature) do
    sign_in('ada')
  end
end
