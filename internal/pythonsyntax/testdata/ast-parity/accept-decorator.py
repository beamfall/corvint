def decorator(*args, **kwargs):
    def wrap(fn):
        return fn
    return wrap


@decorator(x=1, *args, **kwargs)
def handler():
    pass
