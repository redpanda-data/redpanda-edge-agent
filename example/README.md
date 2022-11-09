# Running the example

This [Docker Compose](.compose.yaml) file spins up a local environment for testing the agent. The environment starts two containers:

- `redpanda-source`: simulates an IoT device that runs a single-node Redpanda instance and the agent to store and forward messages
- `redpanda-destination`: simulates a central Redpanda cluster that aggregates messages from all of the IoT devices

## Prerequisites

1. The agent communicates with the source and destinations clusters over TLS enabled interfaces, so before starting the containers, run [generate-certs.sh](./generate-certs.sh) to create the necessary certificates. The resulting `./certs` directory is mounted on the containers
2. Run the [build.sh](./build.sh) script to compile the agent for the Linux-based container

## Testing the agent

Open a terminal and produce some messages to the source's `telemetry` topic:

```bash
export REDPANDA_BROKERS=localhost:19092
for i in {1..60}; do echo $(cat /dev/urandom | head -c10 | base64) | rpk topic produce telemetry; sleep 1; done
```

The agent will forward the messages to a topic with the same name on the destination container. Open a second terminal and consume the messages:

```bash
export REDPANDA_BROKERS=localhost:29092
rpk topic consume telemetry
{
  "topic": "telemetry",
  "key": "51940184cb08",
  "value": "q/F5LP5DmnIPog==",
  "timestamp": 1667984441252,
  "partition": 0,
  "offset": 0
}
{
  "topic": "telemetry",
  "key": "51940184cb08",
  "value": "5ATcnSvzmd3vOw==",
  "timestamp": 1667984442624,
  "partition": 0,
  "offset": 1
}
{
  "topic": "telemetry",
  "key": "51940184cb08",
  "value": "9dtQdCH01KWaNQ==",
  "timestamp": 1667984443669,
  "partition": 0,
  "offset": 2
}
...
```