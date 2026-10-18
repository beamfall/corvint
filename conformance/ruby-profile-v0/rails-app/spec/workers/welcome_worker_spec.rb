require "rails_helper"

RSpec.describe WelcomeWorker do
  it "greets the user" do
    allow(User).to receive(:find).and_return(User.new(name: "ada"))
    expect { described_class.new.perform(1) }.not_to raise_error
  end
end
