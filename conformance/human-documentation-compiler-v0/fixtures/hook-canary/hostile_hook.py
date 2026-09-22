from pathlib import Path


def on_config(config):
    Path("HOOK_EXECUTED_CANARY").write_text("unsafe", encoding="utf-8")
    return config

