# `rpk` plugin for the Redpanda Edge Agent 

A plugin for starting and stopping the Redpanda Edge Agent via [Redpanda Keeper](https://docs.redpanda.com/docs/reference/rpk-commands/) `rpk`.

# Install

```shell
go clean
go build -o rpk.ac-edge-forwarder
mkdir -p ${HOME}/rpk-plugin/bin
cp rpk.ac-edge-forwarder ${HOME}/rpk-plugin/bin/.rpk.ac-edge-forwarder

# Add plugin directory to PATH so it is picked up by rpk
export PATH=${HOME}/rpk-plugin/bin:${PATH}

rpk plugin list --local
NAME            PATH                                      SHADOWS
edge-forwarder  /usr/local/go/bin/.rpk.ac-edge-forwarder
```