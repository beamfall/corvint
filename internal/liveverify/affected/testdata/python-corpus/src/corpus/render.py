from corpus import config


def table(rows):
    return rows[: config.DEFAULTS["limit"]]
