#!/bin/bash
set -o errexit
set -o pipefail
set -e 

if [ -z "$GITHUB_REPOSITORY" ]; then
  echo "Please set env var for GITHUB_REPOSITORY. E.g: BeanCloudServices/iam-keycloak-bcs"
  exit 1
fi

if [ -z "$GITHUB_TOKEN" ]; then
  echo "Please set env var GITHUB_TOKEN to your GITHUB PAT (Personal Access Token)"
  exit 1
fi

if [ -z "$GITHUB_REPOSITORY_OWNER" ]; then
  echo "Please set env var for GITHUB_REPOSITORY_OWNER. E.g: BeanCloudServices"
  exit 1
fi

# SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
SCRIPT_DIR=$(pwd)

usage() {
    THIS=$(basename $0)
    cat << EOF
Usage: 
  \$ $THIS <keycloak major version>

EOF
    echo $1
}

git submodule foreach --recursive 'git fetch --tags --force'

MAJOR_VERSION=$1

# if [ -z "$MAJOR_VERSION" ]; then usage "ERROR | No major version provided"; exit 1; fi
if [ -z "$MAJOR_VERSION" ]; then
  usage "WARNING | No major version provided, fetching the latest release"; MAJOR_VERSION=v; 
  TAG=$(curl --silent "https://api.github.com/repos/keycloak/keycloak/tags" | jq -rc '[.[] | select( .name | contains("nightly") | not )][0].name')
  COMMIT_SHA=$(curl --silent "https://api.github.com/repos/keycloak/keycloak/tags" | jq -rc '[.[] | select( .name | contains("nightly") | not )][0].commit.sha')
  echo -e "\n - Import keycloak latest"
else
  TAG=$(curl --silent "https://api.github.com/repos/keycloak/keycloak/tags" | jq -rc --arg majorVersion "${MAJOR_VERSION:1}" '[.[] | select( .name | contains($majorVersion)  )][0].name')
  COMMIT_SHA=$(curl --silent "https://api.github.com/repos/keycloak/keycloak/tags" | jq -rc --arg majorVersion "${MAJOR_VERSION:1}" '[.[] | select( .name | contains($majorVersion)  )][0].commit.sha')
  echo -e "\n - Import keycloak ${MAJOR_VERSION}"

fi

# find the latest git tag of major version
# TAG=$(curl --silent "https://api.github.com/repos/keycloak/keycloak/tags" | jq -r '.[].name' | grep "${MAJOR_VERSION:1}" | grep -v nightly | head -n 1 || true)



if [ -z "$TAG" ]; then echo -e "\n - ERROR | No git tag of the provided major version"; exit 1; fi

echo -e "\n - Found git tag $TAG in keycloak repository"

arrIN=(${TAG//./ })
MAJOR_VERSION="v${arrIN[0]}"

IS_MAJOR_VERSION_EXISTING=`git submodule status "releases/$MAJOR_VERSION/latest/keycloak"; IS_MAJOR_VERSION_EXISTING=$?; echo $IS_MAJOR_VERSION_EXISTING`

if [ "$IS_MAJOR_VERSION_EXISTING" = "1" ]; then
    echo -e "\n - Major Version does not exist, importing now"
else
    echo -e "\n - $MAJOR_VERSION already exists, check if commit is latest"
    CURRENT_MAJOR_VERSION_COMMIT_SHA=`git submodule status releases/${MAJOR_VERSION}/latest | head -n1 | awk '{print $1;}'`
    if [ "$CURRENT_MAJOR_VERSION_COMMIT_SHA" = "$COMMIT_SHA" ]; then
      echo -e "\n - Already latest, nothing to commit. Exiting now"
      exit
    else
      echo -e "\n - Commit is not latest, importing now"
    fi
fi

# just a placeholder
# CURRENT_LATEST_MAJOR_VERSION=`ls -t releases | head -n1`
# CURRENT_LATEST_MAJOR_VERSION_COMMIT_SHA=`git submodule status releases/${CURRENT_LATEST_MAJOR_VERSION}/latest | head -n1 | awk '{print $1;}'`

echo -e "\n - Create release folder by MAJOR_VERSION"
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

cd $SCRIPT_DIR

NEW_BRANCH="releases/${MAJOR_VERSION}/${TAG}"
echo -e "\n - Create a new branch $NEW_BRANCH"
git checkout -b $NEW_BRANCH
echo -e "\n - Pushing to the new branch"
git add .
git commit -m "IMPORT: v${TAG}"
git push --set-upstream origin $NEW_BRANCH

echo -e "\n - Create a new PR"
curl -L \
  -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer $GITHUB_TOKEN"\
  -H "X-GitHub-Api-Version: 2022-11-28" \
  https://api.github.com/repos/$GITHUB_REPOSITORY/pulls \
  -d "{\"title\":\"IMPORT: v${TAG} \",\"body\":\" \",\"head\":\"$GITHUB_REPOSITORY_OWNER:$NEW_BRANCH\",\"base\":\"master\"}"