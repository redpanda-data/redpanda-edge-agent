#!/usr/bin/env bash

go clean
go build -o redpanda-edge-agent
mkdir -p ${HOME}/rpk-plugin/bin
mv redpanda-edge-agent ${HOME}/rpk-plugin/bin
export PATH=${HOME}/rpk-plugin/bin:${PATH}
