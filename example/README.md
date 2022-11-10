# Edge Agent Example

This [Docker Compose](.compose.yaml) file spins up a local environment for testing the agent. The environment starts two containers:

- `redpanda-source`: simulates an IoT device that runs a single-node Redpanda instance and the agent to store and forward messages
- `redpanda-destination`: simulates a central Redpanda cluster that aggregates messages from all of the IoT devices

## Prerequisites

1. The agent communicates with the source and destinations clusters over TLS enabled interfaces, so before starting the containers, run [generate-certs.sh](./generate-certs.sh) to create the necessary certificates. The resulting `./certs` directory is mounted on the containers in the [compose](./compose.yaml) configuration
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
docker exec -it redpanda_source cat /var/lib/redpanda/data/agent.log

time="2022-11-10T11:29:05Z" level=info msg="Init config from file: /etc/redpanda/agent.yaml"
time="2022-11-10T11:29:05Z" level=info msg="Config: {\"Id\":\"01b9b3ec416f\",\"CreateTopics\":true,\"Source\":{\"BootstrapServers\":\"172.24.1.10:9092\",\"Topics\":[\"telemetry\"],\"ConsumerGroup\":\"01b9b3ec416f\",\"MaxPollRecords\":1000,\"DefaultPartitions\":1,\"DefaultReplication\":1,\"SASL\":{\"SaslMethod\":\"\",\"SaslUsername\":\"\",\"SaslPassword\":\"\"},\"TLS\":{\"Enabled\":true,\"ClientKeyFile\":\"/etc/redpanda/certs/agent.key\",\"ClientCertFile\":\"/etc/redpanda/certs/agent.crt\",\"CaFile\":\"/etc/redpanda/certs/ca.crt\"}},\"Destination\":{\"BootstrapServers\":\"172.24.1.20:9092\",\"DefaultPartitions\":1,\"DefaultReplication\":1,\"SASL\":{\"SaslMethod\":\"\",\"SaslUsername\":\"\",\"SaslPassword\":\"\"},\"TLS\":{\"Enabled\":true,\"ClientKeyFile\":\"/etc/redpanda/certs/agent.key\",\"ClientCertFile\":\"/etc/redpanda/certs/agent.crt\",\"CaFile\":\"/etc/redpanda/certs/ca.crt\"}}}"
time="2022-11-10T11:29:05Z" level=info msg="Created Source client"
time="2022-11-10T11:29:05Z" level=info msg="Source broker: {\"NodeID\":1,\"Port\":9092,\"Host\":\"172.24.1.10\",\"Rack\":null}"
time="2022-11-10T11:29:05Z" level=info msg="Created Destination client"
time="2022-11-10T11:29:05Z" level=info msg="Destination broker: {\"NodeID\":1,\"Port\":9092,\"Host\":\"172.24.1.20\",\"Rack\":null}"
time="2022-11-10T11:29:05Z" level=info msg="Created source topic 'telemetry'"
time="2022-11-10T11:29:05Z" level=info msg="Created destination topic 'telemetry'"
time="2022-11-10T11:29:05Z" level=info msg="Starting to forward records..."
time="2022-11-10T11:29:05Z" level=debug msg="Polling for records..."
time="2022-11-10T11:29:10Z" level=debug msg="Consumed 1 records"
time="2022-11-10T11:29:10Z" level=debug msg="Forwarded 1 records"
time="2022-11-10T11:29:10Z" level=debug msg="Committing offsets: {\"telemetry\":{\"0\":{\"Epoch\":1,\"Offset\":1}}}"
time="2022-11-10T11:29:10Z" level=debug msg="Offsets committed"
time="2022-11-10T11:29:10Z" level=debug msg="Polling for records..."
time="2022-11-10T11:29:11Z" level=debug msg="Consumed 1 records"
time="2022-11-10T11:29:11Z" level=debug msg="Forwarded 1 records"
time="2022-11-10T11:29:11Z" level=debug msg="Committing offsets: {\"telemetry\":{\"0\":{\"Epoch\":1,\"Offset\":2}}}"
time="2022-11-10T11:29:11Z" level=debug msg="Offsets committed"
time="2022-11-10T11:29:11Z" level=debug msg="Polling for records..."
```