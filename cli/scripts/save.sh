#!/bin/bash
MAJOR_VERSION=$1

# if [ -z "$MAJOR_VERSION" ]; then usage "ERROR | No major version provided"; exit 1; fi
if [ -z "$MAJOR_VERSION" ]; then
    echo "Please specify the version which you want to save"
    exit 1
fi
SUB_MODULE_KEYCLOAK_PATH="releases/${MAJOR_VERSION}/latest/keycloak"
TARGET_BRANCH="origin/master"

#.github folder should be copied to cicd folder
echo "copy .github folder to cicd"
cp -r ./.github/ releases/${MAJOR_VERSION}/cicd/

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
git diff --pretty --diff-filter=RM ${TARGET_BRANCH} ${SUB_MODULE_KEYCLOAK_PATH} > releases/${MAJOR_VERSION}/latest/patches/modifications.diff

#File deletions should be saved in patches folder as a removals.diff file.
git diff --pretty --diff-filter=D ${TARGET_BRANCH} ${SUB_MODULE_KEYCLOAK_PATH} > releases/${MAJOR_VERSION}/latest/patches/removals.diff

#Any new files that were added should be copied to overrides folder.
for modification in $(git diff --name-only --pretty --diff-filter=A ${TARGET_BRANCH} ${SUB_MODULE_KEYCLOAK_PATH});
do cp $modification releases/${MAJOR_VERSION}/latest/overrides; done

#When every step above has been done, git keycloak submodule should be git reset --hard to discard all changes.
git restore --source=HEAD --staged --worktree -- ${SUB_MODULE_KEYCLOAK_PATH}
