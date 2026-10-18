module Bar
  class BazService
    def call(id)
      FooClient.new.fetch(id)
    end
  end
end
