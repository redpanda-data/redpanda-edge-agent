# Running locally in Docker

This [Docker Compose](.compose.yaml) file spins up a local environment for testing the agent. The environment starts two containers:

- `redpanda-source`: simulates an IoT device that runs a single-node Redpanda instance and the agent to store and forward events.
- `redpanda-destination`: simulates a central Redpanda cluster that aggregates events from all of the IoT devices

The agent communicates with the source and destinations clusters over TLS enabled interfaces, so before starting the containers, be sure to run [generate-certs.sh](./generate-certs.sh) to create the necessary certificates. You must also run the [build.sh](./build.sh) script to compile the agent for the Linux-based container.

## Demo application

The demo application streams location tracking events to the source Redpanda. It's intended to run alongside Redpanda and the forwarding agent on the IoT device, but it can run anywhere as long as it can connect to the source Redpanda. The events include `latitude`, `longitude`, and `elevation` data that was recorded at one second intervals by the cycling computers used by professional riders during stages of the 2022 Tour de France. The original XML formatted `.gpx` files were downloaded from the rider's public profiles on [Strava](https://www.strava.com/). The application transforms the location events from XML to JSON formatted messages and produces them at a configurable interval (one event every 100ms by default):

```json
{
    "Time":18841,
    "Athlete":"WOUT VAN AERT",
    "Team":"JUMBO - VISMA",
    "Name":"TdF 2022 - Stage 9",
    "Latitude":"46.2337990",
    "Longitude":"6.7904500",
    "Elevation":"1287.4"
}
```

### Building the demo

```shell
cd demo/

# MacOS (Intel)
./build.sh darwin/amd64

# MacOS (M1)
./build.sh darwin/arm64

# Linux (x86_64)
./build.sh linux/amd64

# Linux (Arm)
./build.sh linux/arm64

telemetry-demo -brokers localhost:19092
```
