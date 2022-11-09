#!/usr/bin/env bash
rm -f redpanda-edge-agent
cd ../agent
env GOOS=linux GOARCH=amd64 go build -a -v -o ../example/redpanda-edge-agent
