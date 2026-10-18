class Account < ApplicationRecord
  def owner_name
    User.new.display_name
  end
end
