# Running the example

This [Docker Compose](.compose.yaml) file spins up a local environment for testing the agent. The environment starts two containers:

- `redpanda-source`: simulates an IoT device that runs a single-node Redpanda instance and the agent to store and forward messages
- `redpanda-destination`: simulates a central Redpanda cluster that aggregates messages from all of the IoT devices

## Prerequisites

The following commands are to be ran from this `examples` folder.

1. The agent communicates with the source and destinations clusters over TLS enabled interfaces, so before starting the containers, run [generate-certs.sh](./generate-certs.sh) to create the necessary certificates. The resulting `./certs` directory is mounted on the containers
2. Run the [build.sh](./build.sh) script to compile the agent for the Linux-based container

> Note: On Linux, the `certs` folder and `redpanda-edge-agent` binary will need to be usable by the user running the process within the container. Run the following command:
>
> ```bash
> sudo chmod -R 777 certs redpanda-edge-agent
> ```

## Start Redpanda

In terminal 1, run `docker compose up` to start the `redpanda-source` and `redpanda-destination` containers.

> You may get an error like the following:
>
> ```
> redpanda_destination  | ERROR 2022-11-09 15:29:57,976 [shard 1] seastar - Could not setup Async I/O: Resource temporarily unavailable. The most common cause is not enough request capacity in /proc/sys/fs/aio-max-nr. Try increasing that number or reducing the amount of logical CPUs available for your application
> ```
>
> This can be resolved by running `sudo sysctl fs.aio-max-nr=1048576`. Consider making this permanent by following the instructions [here](https://sort.veritas.com/public/documents/HSO/2.0/linux/productguides/html/hfo_admin_rhel/ch04s03.htm).

## Testing the agent

In terminal 2, produce some messages to the source's `telemetry` topic. The agent will forward the messages to a topic with the same name on the destination container:

```bash
export REDPANDA_BROKERS=localhost:19092
for i in {1..60}; do echo $(cat /dev/urandom | head -c10 | base64) | rpk topic produce telemetry; sleep 1; done
```

In terminal 3, consume messages from the destination's topic:

```bash
export REDPANDA_BROKERS=localhost:29092
rpk topic consume telemetry
```

You should start seeing output similar to the following:

```bash
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
