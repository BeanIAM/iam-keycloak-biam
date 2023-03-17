
echo "Building yq"
YQ_BUILD_PATH="./cli/build/yq"
if [ -d "$YQ_BUILD_PATH" ]; then
echo "... Github repo already cloned"
else
echo "... Cloning Github yq repo"
    git clone git@github.com:BeanOpenSource/yq.git YQ_BUILD_PATH
fi

echo "... Building from source"
cd ./cli/build/yq
env GOOS=darwin GOARCH=amd64 go build -o ../../bin/yq-mac-x86
env GOOS=linux go build -o ../../bin/yq-linux-x86
