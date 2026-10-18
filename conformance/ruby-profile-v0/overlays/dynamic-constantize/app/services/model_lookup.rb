class ModelLookup
  def find(name, id)
    name.constantize.find(id)
  end
end
