require "rails_helper"

RSpec.describe Account do
  it "names its owner" do
    expect(described_class.new.owner_name).to be_a(String)
  end
end
