Rails.application.routes.draw do
  root to: "home#index"
  get "welcome", to: "home#welcome", as: :welcome
  post "submit", to: "forms#create", as: :submit
  namespace :admin do
    scope "v2", as: :v2 do
      resources :posts do
        resources :comments
        member do
          get "preview"
        end
        collection do
          get "search"
        end
      end
    end
  end
  resources :articles, shallow: true do
    resources :notes
  end
  resource :profile
  get "files/*path", to: "files#show", as: :files
  mount "RackApp", at: "/mounted", as: :mounted
  draw :extra
end
