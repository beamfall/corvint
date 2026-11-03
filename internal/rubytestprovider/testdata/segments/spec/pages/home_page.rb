# frozen_string_literal: true

class HomePage < SitePrism::Page
  set_url '/'
  element :query, '#q'
  element :go, '#go'

  def search_for(term)
    query.set(term)
    go.click
  end
end
