#!/bin/bash
set -o errexit
set -o pipefail

# SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
SCRIPT_DIR=$(pwd)

usage() {
    echo $1
    THIS=$(basename $0)
    cat << EOF
Usage: 
  \$ $THIS <keycloak major version>

EOF
}

git submodule foreach --recursive 'git fetch --tags --force'

MAJOR_VERSION=$1

# if [ -z "$MAJOR_VERSION" ]; then usage "ERROR | No major version provided"; exit 1; fi
if [ -z "$MAJOR_VERSION" ]; then usage "WARNING | No major version provided, fetching the latest release"; MAJOR_VERSION=v; fi

echo "Import keycloak ${MAJOR_VERSION}"

# find the latest git tag of major version
TAG=$(curl --silent "https://api.github.com/repos/keycloak/keycloak/tags" | jq -r '.[].name' | grep "${MAJOR_VERSION:1}" | grep -v nightly | head -n 1 || true)

if [ -z "$TAG" ]; then echo "ERROR | No git tag of the provided major version"; exit 1; fi

echo "Found git tag $TAG in keycloak repository"

arrIN=(${TAG//./ })
MAJOR_VERSION="v${arrIN[0]}"


echo "Create release folder by MAJOR_VERSION"
# create folders
mkdir -p "$SCRIPT_DIR/releases/$MAJOR_VERSION/latest"
cd "$SCRIPT_DIR/releases/$MAJOR_VERSION/latest"
mkdir -p "overrides"
touch "overrides/.gitkeep"
mkdir -p "patches"
touch "patches/.gitkeep"
mkdir -p "cicd"
touch "cicd/.gitkeep"

# create submodule
if [ -d "keycloak" ]; then
  cd "keycloak"
  git submodule init .
  git submodule update .
  cd ..
fi
git submodule add -f git@github.com:keycloak/keycloak.git keycloak
cd "keycloak"
git checkout "$TAG"
