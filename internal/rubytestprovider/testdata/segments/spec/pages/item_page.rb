# frozen_string_literal: true

class ItemPage < SitePrism::Page
  set_url '/item{/name}'
  element :price, '#price'

  def price_text
    price.text
  end
end
