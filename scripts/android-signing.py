#!/usr/bin/env python3
"""Restores the Android release key from the ANDROID_SIGNING secret into ~/.config/keduly.

The secret is JSON: keystoreBase64, storePassword, keyAlias, keyPassword. Nothing is printed.
"""
import base64
import json
import os
from pathlib import Path

config = Path.home() / ".config/keduly"
config.mkdir(parents=True, exist_ok=True, mode=0o700)
try:
    signing = json.loads(os.environ["ANDROID_SIGNING"])
    values = {"storeFile": "release.jks", **{k: signing[k] for k in ("storePassword", "keyAlias", "keyPassword")}}
    if os.environ.get("GITHUB_ACTIONS") == "true":
        for name in ("storePassword", "keyPassword"):
            print("::add-mask::" + values[name], flush=True)
    # Reject Java-properties delimiters rather than silently producing another password.
    if any(not isinstance(v, str) or not v or any(c in v for c in "\r\n\\:= ") for v in values.values()):
        raise ValueError()
    key = base64.b64decode(signing["keystoreBase64"], validate=True)
    for name, content in {"release.jks": key, "signing.properties": "".join(f"{k}={v}\n" for k, v in values.items()).encode()}.items():
        target = config / name
        target.write_bytes(content)
        target.chmod(0o600)
except (KeyError, ValueError, TypeError):
    raise SystemExit("ANDROID_SIGNING is missing or malformed") from None
