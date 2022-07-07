# Redpanda Edge Agent

A lightweight internet of things (IoT) agent that runs alongside Redpanda at the edge to forward events to a central Kafka API compatible cluster. The agent is written in Go and uses the [franz-go](https://github.com/twmb/franz-go) Kafka client library.

## Modules

This project is split into two modules. The [agent](./agent/) module contains the code for the event forwarding application, and the [plugin](./plugin/) module contains the code for extending `rpk` to add the ability to start and stop the agent using the `rpk` command line ultility. See the module README files for more details and instructions for building and installing the applications at the edge.

## Running locally in Docker

This [Docker Compose](./docker/compose.yaml) file spins up a local environment for testing the agent. The environment starts two containers:

- `redpanda-source`: Simulates an edge environment that runs a single-node Redpanda instance and the agent to forward events
- `redpanda-destination`: Simulates a central Redpanda cluster that collects the events from all of the agents

The agent communicates with the source and destinations clusters over TLS enabled interfaces, so before starting the containers, be sure to run [generate-certs.sh](./docker/generate-certs.sh) to create the necessary certificates. You must also run the [build.sh](./docker/build.sh) script to compile the agent for the Linux-based container.
