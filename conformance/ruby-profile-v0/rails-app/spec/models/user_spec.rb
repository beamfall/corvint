require "rails_helper"

RSpec.describe User do
  it "strips the display name" do
    expect(described_class.new(name: " ada ").display_name).to eq("ada")
  end
end
