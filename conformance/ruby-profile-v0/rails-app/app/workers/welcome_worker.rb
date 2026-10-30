class WelcomeWorker
  include Sidekiq::Worker

  def perform(user_id)
    Greeter.new.greet(User.find(user_id))
  end
end
