#!/bin/bash
set -o errexit
set -o pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

usage() {
    echo $1
    THIS=$(basename $0)
    cat << EOF
Usage: 
  \$ $THIS <keycloak major version>

EOF
}

MAJOR_VERSION=$1

if [ -z "$MAJOR_VERSION" ]; then usage "ERROR | No major version provided"; exit 1; fi

echo "!!! NOTE: Please execute 'git submodule init & update' before to run this script"

echo "Import keycloak ${MAJOR_VERSION}"

# find the latest git tag of major version
TAG=$(curl --silent "https://api.github.com/repos/keycloak/keycloak/tags" | jq -r '.[].name' | grep "${MAJOR_VERSION:1}" | head -n 1 || true)

echo "Found git tag $TAG in keycloak repository"

if [ -z "$TAG" ]; then echo "ERROR | No git tag of the provided major version"; exit 1; fi

echo "Create release folder of MAJOR_VERSION"
# create folders
mkdir -p "$SCRIPT_DIR/releases/$MAJOR_VERSION/latest"
cd "$SCRIPT_DIR/releases/$MAJOR_VERSION/latest"
mkdir -p "overrides"
touch "overrides/.gitkeep"
mkdir -p "patches"
touch "patches/.gitkeep"
mkdir -p "removals"
touch "removals/.gitkeep"

# create submodule
git submodule add -f git@github.com:keycloak/keycloak.git keycloak
cd "keycloak"
git checkout "$TAG"
