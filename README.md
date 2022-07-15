# Redpanda Edge Agent

A lightweight internet of things (IoT) agent that runs alongside Redpanda at the edge to forward events to a central Kafka API compatible cluster. The agent is written in Go and uses the [franz-go](https://github.com/twmb/franz-go) Kafka client library.

<p align="center">
<img src="./redpanda_iot.png" />
</p>

## Modules

This project is split into two modules:

* [agent](./agent/): the event forwarding application.
* [plugin](./plugin/): `rpk` plugin that adds the ability to start and stop the agent using Redpanda's command line tool.

See the module README files for more details and instructions for building and installing the applications at the edge.

The [docker](./docker/) directory contains a [compose.yaml](./docker/compose.yaml) file that spins up a local environment for testing the agent. See the [README](./docker/README.md) for instructions on how to start the containers and run the demo application.
