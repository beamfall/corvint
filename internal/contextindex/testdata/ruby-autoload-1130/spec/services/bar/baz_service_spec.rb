require "rails_helper"

RSpec.describe Bar::BazService do
  it "delegates" do
    expect(described_class.new.call(2)).to eq(id: 2)
  end
end
