# frozen_string_literal: true

# Calc is the fixture library.
class Calc
  def add(a, b)
    a + b
  end

  def diff(a, b)
    if a > b
      a - b
    else
      b - a
    end
  end

  def unused
    :never
  end
end
