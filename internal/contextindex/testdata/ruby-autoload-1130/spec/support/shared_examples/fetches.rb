RSpec.shared_examples "a fetcher" do
  it "returns the id" do
    expect(subject.fetch(build_id)).to eq(id: build_id)
  end
end
