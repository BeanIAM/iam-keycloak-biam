#!/bin/bash
MAJOR_VERSION=$1

# if [ -z "$MAJOR_VERSION" ]; then usage "ERROR | No major version provided"; exit 1; fi
if [ -z "$MAJOR_VERSION" ]; then
    echo "Please specify the version which you want to save"
    exit 1
fi
SUB_MODULE_KEYCLOAK_PATH="releases/${MAJOR_VERSION}/latest/keycloak"
echo "Submodule path: " ${SUB_MODULE_KEYCLOAK_PATH}
TARGET_BRANCH="origin/master"

#.github folder should be copied to cicd folder
echo "copy upstream .github folder to cicd"
LATEST_RELEASE_PATH=releases/${MAJOR_VERSION}/latest
#cp -r ${LATEST_RELEASE_PATH}/keycloak/.github ${LATEST_RELEASE_PATH}/cicd/

# navigate to submodule
cd ${SUB_MODULE_KEYCLOAK_PATH}
echo "Check git diff"
#reference
  #A Added
  #C Copied
  #D Deleted
  #M Modified
  #R Renamed
  #T have their type (mode) changed
  #U Unmerged
  #X Unknown
  #B have had their pairing Broken
  #* All-or-none
#Modifications to managed files as well as File renaming should be saved in patches folder as a modifications.diff file.
git diff --submodule=diff --pretty --diff-filter=RM HEAD > ../patches/modifications.diff

#File deletions should be saved in patches folder as a removals.diff file.
git diff --submodule=diff --pretty --diff-filter=D HEAD > ../patches/removals.diff

#Any new files that were added should be copied to overrides folder.
git add .
for modification in $(git diff --submodule=diff --name-only --pretty --diff-filter=A HEAD);
do
  rsync -avz --relative "$modification" ../overrides;
done
for file in $(git diff master --submodule=submodule-name | grep '^diff --git' | cut -d' ' -f3); do cp "$file" dest-folder; done

#When every step above has been done, git keycloak submodule should be git reset --hard to discard all changes.
git reset --hard HEAD
