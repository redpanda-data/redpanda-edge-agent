#!/usr/bin/env bash

go clean
go build -o rpk.ac-edge-forwarder
mkdir -p ${HOME}/rpk-plugin/bin
mv rpk.ac-edge-forwarder ${HOME}/rpk-plugin/bin/.rpk.ac-edge-forwarder

# Add plugin directory to PATH so it is picked up by rpk
export PATH=${HOME}/rpk-plugin/bin:${PATH}

rpk plugin list --local
