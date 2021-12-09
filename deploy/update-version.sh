#!/usr/bin/env sh

VERSION_FILE="${1:?'Version file must be provided'}"

# Required environment variables
: ${BUILD_VERSION:?'Version must be supplied'}
: ${BUILD_GIT_COMMIT:?'CI Git commit must be supplied'}
: ${BUILD_REF:?'CI build reference ID must be supplied'}

# Optional environment variables
: ${BUILD_DATE:="$(date '+%FT%T%:z')"}

cat <<EOF > "$VERSION_FILE"
version: ${BUILD_VERSION}
buildGitCommit: ${BUILD_GIT_COMMIT}
buildDate: ${BUILD_DATE}
buildRef: ${BUILD_REF}
EOF
