MAJOR_VERSION=$1

# if [ -z "$MAJOR_VERSION" ]; then usage "ERROR | No major version provided"; exit 1; fi
if [ -z "$MAJOR_VERSION" ]; then
    echo "Please specify the version which you want to remove"
    exit 1
fi

git rm releases/${MAJOR_VERSION}/latest/keycloak
rm -rf .git/modules/releases/${MAJOR_VERSION}/latest/keycloak
git config --remove-section submodule.releases/${MAJOR_VERSION}/latest/keycloak

