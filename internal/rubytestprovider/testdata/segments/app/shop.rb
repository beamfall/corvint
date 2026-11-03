# frozen_string_literal: true

require 'rack'

# A small shop the MCJ-V0-013 Ruby segment fixture drives through Capybara.
module Shop
  ITEMS = %w[widget gadget gizmo].freeze

  module Pricing
    def self.price_for(name)
      if name == 'gadget'
        450
      else
        300
      end
    end
  end

  module Catalog
    def self.search(query)
      ITEMS.select { |item| item.include?(query) }
    end
  end

  class App
    def call(env)
      req = Rack::Request.new(env)
      case req.path
      when '/' then html('<form action="/search"><input id="q" name="q"><button id="go">Go</button></form>')
      when '/search' then results(req.params['q'].to_s)
      when '/login' then login(req.params['name'].to_s)
      when '/cart' then cart(req.params['add'].to_s)
      when '/reset' then html('<p id="reset">reset</p>')
      when '/warm' then html('<p id="warm">warm</p>')
      else item(req.path.delete_prefix('/item/'))
      end
    end

    private

    def results(query)
      rows = Catalog.search(query).map { |item| "<li class=\"item\">#{item}</li>" }
      html("<ul>#{rows.join}</ul>")
    end

    def login(name)
      html("<p id=\"user\">#{name}</p>")
    end

    def cart(name)
      html("<p id=\"cart\">#{name}: #{Pricing.price_for(name)}</p>")
    end

    def item(name)
      html("<p id=\"price\">#{Pricing.price_for(name)}</p>")
    end

    def html(body)
      [200, { 'content-type' => 'text/html' }, ["<html><body>#{body}</body></html>"]]
    end
  end
end
