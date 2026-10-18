require "rails_helper"

RSpec.describe "users", type: :request do
  it "lists users" do
    sign_in_as(User.new(name: "ada"))
    get "/users"
    expect(response).to have_http_status(:ok)
  end
end
