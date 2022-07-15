#!/usr/bin/env bash

platform=$1
if [[ -z "$platform" ]]; then
    platform=$(go env GOOS)/$(go env GOARCH)
    echo ${platform}
fi

binary="telemetry-demo"

split=(${platform//\// })
rm -f ${binary}
env GOOS=${split[0]} GOARCH=${split[1]} go build -v -o ${binary} .
if [ ! -f "${binary}" ]; then
    echo "Error building file: ${binary}"
    exit 1
fi

chmod +x ${binary}
archive="${binary}-${split[0]}-${split[1]}.zip"
echo "Saving to: ${archive}"
rm -f ${archive}
zip ${archive} ${binary} ./data/*
echo "Complete!"