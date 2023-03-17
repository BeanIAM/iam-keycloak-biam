#!/bin/bash
MAJOR_VERSION=$1

# if [ -z "$MAJOR_VERSION" ]; then usage "ERROR | No major version provided"; exit 1; fi
if [ -z "$MAJOR_VERSION" ]; then
    echo "Please specify the version which you want to save"
    exit 1
fi

RELEASE_LATEST_PATH="releases/${MAJOR_VERSION}/latest"

#go to keyloak folder
cd "${RELEASE_LATEST_PATH}"/keycloak

#patches/modifications.diff is applied back to keycloak
#need to check whether file exist
git apply ../patches/modifications.diff
#need to check whether file exist
#patches/removals.diff is applied back to keycloak
git apply ../patches/removals.diff
#need to check whether folder is empty
#files and folders in overrides are copied to keycloak
cp -R ../overrides/ ./