module SpecHelpers
  def build_id
    42
  end
end
RSpec.configure { |c| c.include SpecHelpers }
