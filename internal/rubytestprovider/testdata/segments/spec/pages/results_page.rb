# frozen_string_literal: true

class ResultsPage < SitePrism::Page
  set_url '/search'
  elements :items, '.item'

  def titles
    items.map(&:text)
  end
end
