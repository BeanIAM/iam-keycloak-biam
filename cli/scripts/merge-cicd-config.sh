unameOut="$(uname -s)"
case "${unameOut}" in
    Linux*)     machine=linux;;
    Darwin*)    machine=mac;;
    CYGWIN*)    machine=cygwin;;
    MINGW*)     machine=minGw;;
    *)          machine="UNKNOWN:${unameOut}"
esac
echo ${machine}

MAJOR_VERSION=$1

if [ -z "${MAJOR_VERSION}" ]; then 
    MAJOR_VERSION=master
fi

# This simply doesn't work
# ./cli/yaml-merge/bin/yaml-merge-${machine} $MAJOR_VERSION

LATEST_RELEASE_PATH=releases/${MAJOR_VERSION}/latest

if [ "$MAJOR_VERSION" = "master" ]; then
    LATEST_RELEASE_PATH=master
else
      echo -e "\n - Commit is not latest, importing now"
fi


./cli/bin/yq-mac-x86 eval-all "select(fi == 0) * select(filename == \"./${LATEST_RELEASE_PATH}/patches/.github/workflows/ci.yml\")" ./${LATEST_RELEASE_PATH}/keycloak/.github/workflows/ci.yml ./${LATEST_RELEASE_PATH}/patches/.github/workflows/ci.yml > ./${LATEST_RELEASE_PATH}/build/.github/workflows/ci.step1.yml

./cli/bin/yq-mac-x86 'del(.on.schedule)' ./${LATEST_RELEASE_PATH}/build/.github/workflows/ci.step1.yml  > ./${LATEST_RELEASE_PATH}/build/.github/workflows/ci.yml

cp ${LATEST_RELEASE_PATH}/build/.github/workflows/ci.yml .github/workflows/${MAJOR_VERSION}-ci.yml

mkdir -p .github/actions/${MAJOR_VERSION}/

cp -R ./${LATEST_RELEASE_PATH}/keycloak/.github/actions/*  .github/actions/${MAJOR_VERSION}/

# Adapted from https://stackoverflow.com/questions/1583219/how-can-i-do-a-recursive-find-replace-of-a-string-with-awk-or-sed
find ./.github/actions/${MAJOR_VERSION} \( -type d -name .git -prune \) -o -type f -print0 | xargs -0 sed -i "" "s/.github\/actions/.github\/actions\/${MAJOR_VERSION}/g" 

sed -i "" "s/.github\/actions/.github\/actions\/${MAJOR_VERSION}/g" .github/workflows/${MAJOR_VERSION}-ci.yml