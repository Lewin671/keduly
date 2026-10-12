#!/bin/sh
# Builds the Android app and runs its unit tests and lint. Needs JDK 17 and the Android SDK.
set -eu
cd "$(dirname "$0")/.."

if [ -z "${JAVA_HOME:-}" ] && [ -x /usr/libexec/java_home ]; then
  JAVA_HOME=$(/usr/libexec/java_home -v 17)
  export JAVA_HOME
fi
if [ -z "${ANDROID_HOME:-}" ]; then
  for dir in "$HOME/Library/Android/sdk" "$HOME/Android/Sdk"; do
    if [ -d "$dir" ]; then
      ANDROID_HOME=$dir
      export ANDROID_HOME
      break
    fi
  done
fi

./android/gradlew -p android :app:testDebugUnitTest :app:lintDebug :app:assembleDebug

echo "android checks passed"
