#!/usr/bin/env python3
"""The release's version and the manifest the Android app reads to update itself.

  scripts/release-manifest.py check TAG          the tag, version.properties and ios/project.yml agree
  scripts/release-manifest.py manifest TAG APK   prints latest.properties for the APK of TAG

The release notes in the manifest are the tag's annotation, written for users.
"""
import hashlib
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent


def product():
    values = dict(line.split("=", 1) for line in (ROOT / "version.properties").read_text().split())
    version, code = values["version"], values["androidVersionCode"]
    if not re.fullmatch(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", version) or not re.fullmatch(r"[1-9]\d*", code):
        sys.exit(f"version.properties is malformed: version={version} androidVersionCode={code}")
    return version, code


def check(tag):
    version, code = product()
    if tag != f"v{version}":
        sys.exit(f"tag {tag} does not match version.properties ({version})")
    project = (ROOT / "ios/project.yml").read_text()
    for key, want in (("MARKETING_VERSION", version), ("CURRENT_PROJECT_VERSION", code)):
        found = re.search(rf'^\s*{key}:\s*"?([^"\n]+)"?\s*$', project, re.M)
        if not found or found[1] != want:
            sys.exit(f"ios/project.yml {key} must be {want}")
    return version, code


def escape(value):
    # java.util.Properties: one logical line, backslashes and line breaks escaped.
    return value.replace("\\", "\\\\").replace("\r", "").replace("\n", "\\n")


def manifest(tag, apk):
    version, code = check(tag)
    data = Path(apk).read_bytes()
    notes = subprocess.run(["git", "tag", "--list", tag, "--format=%(contents)"], cwd=ROOT, capture_output=True, text=True, check=True).stdout
    # A lightweight tag's "contents" is the commit message, which is not written for users.
    if subprocess.run(["git", "cat-file", "-t", tag], cwd=ROOT, capture_output=True, text=True, check=True).stdout.strip() != "tag":
        notes = ""
    notes = re.sub(r"-----BEGIN PGP SIGNATURE-----.*", "", notes, flags=re.S).strip()
    fields = {
        "version": version,
        "versionCode": code,
        "apk": f"keduly-android-{tag}.apk",
        "sha256": hashlib.sha256(data).hexdigest(),
        "size": str(len(data)),
        "notes": notes,
    }
    print("".join(f"{k}={escape(v)}\n" for k, v in fields.items()), end="")


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "check":
        check(sys.argv[2])
    elif len(sys.argv) == 4 and sys.argv[1] == "manifest":
        manifest(sys.argv[2], sys.argv[3])
    else:
        sys.exit(__doc__)
