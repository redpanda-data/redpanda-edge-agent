# Edge Agent Example

This [Docker Compose](.docker-compose-redpanda.yaml) file spins up a local environment for testing the agent. The environment starts two containers:

- `redpanda-source`: simulates an IoT device that runs a single-node Redpanda instance and the agent to store and forward messages
- `redpanda-destination`: simulates a central Redpanda cluster that aggregates messages from all of the IoT devices

## Prerequisites

1. The agent communicates with the source and destinations clusters over TLS enabled interfaces, so before starting the containers, run [generate-certs.sh](./generate-certs.sh) to create the necessary certificates. The resulting `./certs` directory is mounted on the containers in the [compose](./compose.yaml) configuration. *Note: On Linux, it may be necessary to run `sudo chmod -R 777 certs` to ensure this directory can be read by the container user `redpanda`.*
2. Run the [build.sh](./build.sh) script to compile the agent for the Linux-based container

## Start the containers

```bash
cd example
docker-compose -f docker-compose-redpanda.yaml up -d
[+] Running 3/3
 ⠿ Network example_redpanda_network  Created
 ⠿ Container redpanda_source         Started
 ⠿ Container redpanda_destination    Started
```

## Test the agent

Open a new terminal and produce some messages to the source's `telemetryB` topic (note that the example [agent](./agent.yaml) is configured to create the topics on startup):

```bash
export REDPANDA_BROKERS=localhost:19092
for i in {1..60}; do echo $(cat /dev/urandom | head -c10 | base64) | rpk topic produce telemetryB; sleep 1; done
```

The agent will forward the messages to a topic with the same name on the destination. Open a second terminal and consume the messages:

```bash
export REDPANDA_BROKERS=localhost:29092
rpk topic consume telemetryC
{
  "topic": "telemetryC",
  "key": "a0f1fd421b85",
  "value": "aZ7NEkkd977GXQ==",
  "timestamp": 1674753398569,
  "partition": 0,
  "offset": 0
}
{
  "topic": "telemetryC",
  "key": "a0f1fd421b85",
  "value": "cJRS+n9mJCAxzg==",
  "timestamp": 1674753399965,
  "partition": 0,
  "offset": 1
}
{
  "topic": "telemetryC",
  "key": "a0f1fd421b85",
  "value": "sNPcw6YWBpQl9g==",
  "timestamp": 1674753401025,
  "partition": 0,
  "offset": 2
}
...
```

## Tail the agent log

```bash
docker exec redpanda_source tail -100f /var/lib/redpanda/data/agent.log
time="2023-01-26T17:16:15Z" level=info msg="Init config from file: /etc/redpanda/agent.yaml"
time="2023-01-26T17:16:15Z" level=debug msg="create_topics -> true\ndestination.bootstrap_servers -> 172.24.1.20:9092\ndestination.consumer_group_id -> a0f1fd421b85\ndestination.max_version -> 3.0.0\ndestination.name -> destination\ndestination.tls.ca_cert -> /etc/redpanda/certs/ca.crt\ndestination.tls.client_cert -> /etc/redpanda/certs/agent.crt\ndestination.tls.client_key -> /etc/redpanda/certs/agent.key\ndestination.tls.enabled -> true\ndestination.topics -> [configA configB:configA]\nid -> a0f1fd421b85\nmax_backoff_secs -> 600\nmax_poll_records -> 1000\nsource.bootstrap_servers -> 172.24.1.10:9092\nsource.consumer_group_id -> a0f1fd421b85\nsource.name -> source\nsource.tls.ca_cert -> /etc/redpanda/certs/ca.crt\nsource.tls.client_cert -> /etc/redpanda/certs/agent.crt\nsource.tls.client_key -> /etc/redpanda/certs/agent.key\nsource.tls.enabled -> true\nsource.topics -> [telemetryA telemetryB:telemetryC]\n"
time="2023-01-26T17:16:15Z" level=info msg="Added push topic: telemetryA > telemetryA"
time="2023-01-26T17:16:15Z" level=info msg="Added push topic: telemetryB > telemetryC"
time="2023-01-26T17:16:15Z" level=info msg="Created source client"
time="2023-01-26T17:16:15Z" level=debug msg="source broker: {\"NodeID\":0,\"Port\":9092,\"Host\":\"172.24.1.10\",\"Rack\":null}"
time="2023-01-26T17:16:15Z" level=info msg="Added pull topic: configA < configA"
time="2023-01-26T17:16:15Z" level=info msg="Added pull topic: configA < configB"
time="2023-01-26T17:16:15Z" level=info msg="Created destination client"
time="2023-01-26T17:16:15Z" level=debug msg="destination broker: {\"NodeID\":0,\"Port\":9092,\"Host\":\"172.24.1.20\",\"Rack\":null}"
time="2023-01-26T17:16:15Z" level=info msg="Created topic 'telemetryA' on source"
time="2023-01-26T17:16:15Z" level=info msg="Created topic 'telemetryB' on source"
time="2023-01-26T17:16:15Z" level=info msg="Created topic 'configA' on source"
time="2023-01-26T17:16:15Z" level=info msg="Created topic 'telemetryA' on destination"
time="2023-01-26T17:16:15Z" level=info msg="Created topic 'telemetryC' on destination"
time="2023-01-26T17:16:15Z" level=info msg="Created topic 'configA' on destination"
time="2023-01-26T17:16:15Z" level=info msg="Created topic 'configB' on destination"
time="2023-01-26T17:16:15Z" level=info msg="Forwarding records from 'destination' to 'source'" id=destination
time="2023-01-26T17:16:15Z" level=info msg="Forwarding records from 'source' to 'destination'" id=source
time="2023-01-26T17:16:15Z" level=debug msg="Polling for records..." id=source
time="2023-01-26T17:16:15Z" level=debug msg="Polling for records..." id=destination
time="2023-01-26T17:16:38Z" level=debug msg="Consumed 1 records" id=source
time="2023-01-26T17:16:38Z" level=debug msg="Mapping topic name 'telemetryB' to 'telemetryC'" id=source
time="2023-01-26T17:16:39Z" level=debug msg="Sent 1 records to 'destination'" id=source
time="2023-01-26T17:16:39Z" level=debug msg="Committing offsets: {\"telemetryB\":{\"0\":{\"Epoch\":1,\"Offset\":1}}}" id=source
time="2023-01-26T17:16:39Z" level=debug msg="Offsets committed" id=source
time="2023-01-26T17:16:39Z" level=debug msg="Polling for records..." id=source
...
```
