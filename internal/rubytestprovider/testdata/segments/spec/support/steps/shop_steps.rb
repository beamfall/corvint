# frozen_string_literal: true

# Shared steps: the Ruby collector bounds each outermost call of a method
# defined under spec/support/steps as one step segment.
module ShopSteps
  def sign_in(name)
    visit "/login?name=#{name}"
  end

  def add_to_cart(name)
    visit "/cart?add=#{name}"
    find('#cart').text
  end
end
