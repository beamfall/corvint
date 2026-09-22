def decorator(*args, **kwargs):
    def wrap(fn):
        return fn
    return wrap


@decorator(x=1, 2)
def handler():
    pass
