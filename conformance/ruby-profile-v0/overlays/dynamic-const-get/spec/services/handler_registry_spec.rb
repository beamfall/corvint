require "rails_helper"

RSpec.describe HandlerRegistry do
  it "builds a worker by name" do
    expect(described_class.new.handler("WelcomeWorker")).to be_a(WelcomeWorker)
  end
end
