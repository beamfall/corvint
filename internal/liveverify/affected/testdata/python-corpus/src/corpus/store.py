from corpus import hashing, paths


class Store:
    def key(self, data):
        return hashing.digest(data), paths.root()
