#!/bin/sh
# Builds the iOS app and runs its unit tests on a simulator.
# Needs Xcode and XcodeGen (https://github.com/yonaskolb/XcodeGen).
set -eu
cd "$(dirname "$0")/../ios"

if ! command -v xcodegen >/dev/null 2>&1; then
  echo "xcodegen is not installed" >&2
  exit 1
fi
xcodegen generate --quiet

# Any available iPhone simulator will do; one that is already booted comes first.
udid=$(xcrun simctl list devices available | awk '
  /^-- / { ios = ($0 ~ /iOS/) }
  ios && /iPhone/ {
    if (match($0, /[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}/)) {
      id = substr($0, RSTART, RLENGTH)
      if ($0 ~ /Booted/) { print id; found = 1; exit }
      if (first == "") first = id
    }
  }
  END { if (!found && first != "") print first }')
if [ -z "$udid" ]; then
  echo "no iPhone simulator is available" >&2
  exit 1
fi

xcodebuild \
  -project Keduly.xcodeproj \
  -scheme Keduly \
  -destination "platform=iOS Simulator,id=$udid" \
  -derivedDataPath build \
  CODE_SIGNING_ALLOWED=NO \
  test

echo "iOS checks passed"
