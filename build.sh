#!/usr/bin/env bash

version=$1
if [[ -z "$version" ]]; then
    echo "usage: $0 <version>"
    exit 1
fi
platforms=("darwin/amd64" "darwin/arm64" "linux/amd64" "linux/arm64")
agent_file="redpanda-edge-agent"

for i in "${platforms[@]}"
do
    echo "Building release for ${i}"
    platform=(${i//\// })
    
    rm -f ${agent_file}
    env GOOS=${platform[0]} GOARCH=${platform[1]} go build -a -v -o ${agent_file} ./agent/
    if [ ! -f "${agent_file}" ]; then
        echo "Error building file: ${agent_file}"
        exit 1
    fi

    archive="redpanda-edge-agent-${platform[0]}-${platform[1]}-${version}.tar.gz"
    echo "Saving to: ${archive}"
    rm -f ${archive}
    tar -czvf ${archive} ${agent_file}
done

rm -f ${agent_file}
echo "Complete!"