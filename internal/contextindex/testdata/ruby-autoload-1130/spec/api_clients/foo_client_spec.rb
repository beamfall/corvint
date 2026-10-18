require "rails_helper"

RSpec.describe FooClient do
  subject { described_class.new }
  it_behaves_like "a fetcher"
  it "fetches" do
    expect(described_class.new.fetch(1)).to eq(id: 1)
  end
end
