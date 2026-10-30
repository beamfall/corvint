class HandlerRegistry
  def handler(name)
    Object.const_get(name).new
  end
end
