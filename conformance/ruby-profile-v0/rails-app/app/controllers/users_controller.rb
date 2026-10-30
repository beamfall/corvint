class UsersController < ApplicationController
  def index
    render json: User.all.map(&:display_name)
  end

  def show
    render json: { name: User.find(params[:id]).display_name }
  end
end
