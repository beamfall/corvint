RSpec.shared_examples "a greeter" do
  it "greets by display name" do
    person = double(display_name: "ada")
    expect(described_class.new.greet(person)).to include("ada")
  end
end
