require "rails_helper"

RSpec.describe ModelLookup do
  it "finds a record by model name" do
    allow(User).to receive(:find).and_return(User.new(name: "ada"))
    expect(described_class.new.find("user".classify, 1).display_name).to eq("ada")
  end
end
