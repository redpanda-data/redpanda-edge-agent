# Edge Agent Example

This [Docker Compose](.compose.yaml) file spins up a local environment for testing the agent. The environment starts two containers:

- `redpanda-source`: simulates an IoT device that runs a single-node Redpanda instance and the agent to store and forward messages
- `redpanda-destination`: simulates a central Redpanda cluster that aggregates messages from all of the IoT devices

## Prerequisites

1. The agent communicates with the source and destinations clusters over TLS enabled interfaces, so before starting the containers, run [generate-certs.sh](./generate-certs.sh) to create the necessary certificates. The resulting `./certs` directory is mounted on the containers in the [compose](./compose.yaml) configuration. *Note: On Linux, it may be necessary to run `sudo chmod -R 777 certs` to ensure this directory can be read by the container user `redpanda`.*
2. Run the [build.sh](./build.sh) script to compile the agent for the Linux-based container

## Start the containers

```bash
cd example
docker-compose up -d
[+] Running 3/3
 ⠿ Network example_redpanda_network  Created
 ⠿ Container redpanda_source         Started
 ⠿ Container redpanda_destination    Started
```

## Test the agent

Open a new terminal and produce some messages to the source's `telemetry` topic (note that the example [agent](./agent.yaml) is configured to create the topics on startup):

```bash
export REDPANDA_BROKERS=localhost:19092
for i in {1..60}; do echo $(cat /dev/urandom | head -c10 | base64) | rpk topic produce telemetry; sleep 1; done
```

The agent will forward the messages to a topic with the same name on the destination. Open a second terminal and consume the messages:

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

## Tail the agent log

```bash
docker exec redpanda_source tail -100f /var/lib/redpanda/data/agent.log

time="2022-11-18T10:07:12Z" level=info msg="Init config from file: /etc/redpanda/agent.yaml"
time="2022-11-18T10:07:12Z" level=debug msg="create_topics -> true\ndestination.bootstrap_servers -> 172.24.1.20:9092\ndestination.consumer_group_id -> 32f8d5c415cb\ndestination.name -> destination\ndestination.tls.ca_cert -> /etc/redpanda/certs/ca.crt\ndestination.tls.client_cert -> /etc/redpanda/certs/agent.crt\ndestination.tls.client_key -> /etc/redpanda/certs/agent.key\ndestination.tls.enabled -> true\ndestination.topics -> [config1 config2]\nid -> 32f8d5c415cb\nmax_backoff_secs -> 600\nmax_poll_records -> 1000\nsource.bootstrap_servers -> 172.24.1.10:9092\nsource.consumer_group_id -> 32f8d5c415cb\nsource.name -> source\nsource.tls.ca_cert -> /etc/redpanda/certs/ca.crt\nsource.tls.client_cert -> /etc/redpanda/certs/agent.crt\nsource.tls.client_key -> /etc/redpanda/certs/agent.key\nsource.tls.enabled -> true\nsource.topics -> [telemetry1 telemetry2]\n"
time="2022-11-18T10:07:12Z" level=info msg="Created source client"
time="2022-11-18T10:07:12Z" level=info msg="\t source broker: {\"NodeID\":1,\"Port\":9092,\"Host\":\"172.24.1.10\",\"Rack\":null}"
time="2022-11-18T10:07:12Z" level=info msg="Created destination client"
time="2022-11-18T10:07:12Z" level=info msg="\t destination broker: {\"NodeID\":1,\"Port\":9092,\"Host\":\"172.24.1.20\",\"Rack\":null}"
time="2022-11-18T10:07:12Z" level=info msg="Created topic 'telemetry1' on source"
time="2022-11-18T10:07:12Z" level=info msg="Created topic 'telemetry2' on source"
time="2022-11-18T10:07:12Z" level=info msg="Created topic 'config1' on source"
time="2022-11-18T10:07:12Z" level=info msg="Created topic 'config2' on source"
time="2022-11-18T10:07:12Z" level=info msg="Created topic 'telemetry1' on destination"
time="2022-11-18T10:07:12Z" level=info msg="Created topic 'telemetry2' on destination"
time="2022-11-18T10:07:13Z" level=info msg="Created topic 'config1' on destination"
time="2022-11-18T10:07:13Z" level=info msg="Created topic 'config2' on destination"
time="2022-11-18T10:07:13Z" level=info msg="Forwarding records from 'destination' to 'source'"
time="2022-11-18T10:07:13Z" level=debug msg="Polling for records..."
time="2022-11-18T10:07:13Z" level=info msg="Forwarding records from 'source' to 'destination'"
time="2022-11-18T10:07:13Z" level=debug msg="Polling for records..."
time="2022-11-18T10:08:24Z" level=debug msg="Consumed 1 records"
time="2022-11-18T10:08:25Z" level=debug msg="Forwarded 1 records"
time="2022-11-18T10:08:25Z" level=debug msg="Committing offsets: {\"telemetry1\":{\"0\":{\"Epoch\":1,\"Offset\":1}}}"
time="2022-11-18T10:08:25Z" level=debug msg="Offsets committed"
time="2022-11-18T10:08:25Z" level=debug msg="Polling for records..."
time="2022-11-18T10:08:25Z" level=debug msg="Consumed 1 records"
time="2022-11-18T10:08:25Z" level=debug msg="Forwarded 1 records"
time="2022-11-18T10:08:25Z" level=debug msg="Committing offsets: {\"telemetry1\":{\"0\":{\"Epoch\":1,\"Offset\":2}}}"
time="2022-11-18T10:08:25Z" level=debug msg="Offsets committed"
time="2022-11-18T10:08:25Z" level=debug msg="Polling for records..."
```
