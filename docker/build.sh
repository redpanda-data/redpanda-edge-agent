#!/usr/bin/env bash

rm -f redpanda-edge-agent
rm -f rpk.ac-edge-forwarder

env GOOS=linux GOARCH=amd64 go build -v -o redpanda-edge-agent ../agent
env GOOS=linux GOARCH=amd64 go build -v -o rpk.ac-edge-forwarder ../plugin
