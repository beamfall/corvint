class User < ApplicationRecord
  def display_name
    name.to_s.strip
  end
end
