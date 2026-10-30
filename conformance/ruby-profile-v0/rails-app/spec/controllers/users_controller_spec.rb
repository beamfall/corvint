require "rails_helper"

RSpec.describe UsersController, type: :controller do
  it "shows a user" do
    allow(User).to receive(:find).and_return(User.new(name: "ada"))
    get :show, params: { id: 1 }
    expect(response).to have_http_status(:ok)
  end
end
