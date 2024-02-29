<div align="center">
  <img width="15%" src="./redpanda_lab1.png" />
  <br />
  This is a <a href="https://github.com/redpanda-data/redpanda-labs">Redpanda Labs</a> project
</div>

# Redpanda Edge Agent

A lightweight Internet of Things (IoT) agent that runs alongside Redpanda at the edge to forward events to a central Kafka API compatible cluster. The agent is written in Go and uses the [franz-go](https://github.com/twmb/franz-go) Kafka client library.

<p align="center">
  <img width="50%" src="./redpanda_iot.png" />
</p>

# Build the agent

Build the agent for any Go-supported target platform by setting the `GOOS` and `GOARCH` variables. For a full list of supported architectures, run: `go tool dist list`.

```shell
go clean

# MacOS (Intel)
env GOOS=darwin GOARCH=amd64 go build -a -v -o redpanda-edge-agent ./agent

# MacOS (M1)
env GOOS=darwin GOARCH=arm64 go build -a -v -o redpanda-edge-agent ./agent

# Linux (x86_64)
env GOOS=linux GOARCH=amd64 go build -a -v -o redpanda-edge-agent ./agent

# Linux (Arm)
env GOOS=linux GOARCH=arm64 go build -a -v -o redpanda-edge-agent ./agent
```

The [build.sh](./build.sh) script builds the agent and adds the resulting executable to a tarball for the platforms listed above.

# Running the agent

```shell
Usage of ./redpanda-edge-agent:
  -config string
    	path to agent config file (default "agent.yaml")
  -loglevel string
    	logging level (default "info")
```

# Configuration

Example `agent.yaml`:

```yaml
# The unique identifier for the agent. If not specified the id defaults to the
# hostname reported by the kernel. When forwarding a record, if the record's
# key is empty, then it is set to the agent id to ensure that all records sent
# by the same agent are routed to the same destination topic partition.
id:
# If the provided topics do not exist, attempt to create them on the source and
# destination clusters.
create_topics: true
# Source cluster configuration. The agent will typically run as a sidecar to
# the source cluster in an edge environment.
source:
    # Name of the source cluster
    name: "source"
    # List of Redpanda nodes (typically a single node at the edge).
    bootstrap_servers: 127.0.0.1:19092
    # List of topics to forward to the destination cluster.
    #   - Specify a single topic name to forward events to a topic with the same
    #     name on the destination cluster (e.g. "telemetryA").
    #   - Specify a pair of topic names, separated by a colon, to forward events
    #     to a topic with a different name on the destination cluster
    #     (e.g. "telemetryB:telemetryC").
    topics:
        - telemetryA
        - telemetryB:telemetryC
    # Set the consumer group for the agent to join and consume in. Defaults to
    # the agent id if not set.
    consumer_group_id: ""
    # The source cluster TLS configuration.
    tls:
        enabled: true
        client_key: "agent.key"
        client_cert: "agent.crt"
        ca_cert: "ca.crt"
    # The source cluster SASL configuration.
    sasl:
        # Valid SASL methods: PLAIN | SCRAM-SHA-256 | SCRAM-SHA-512
        sasl_method: ""
        sasl_username: ""
        sasl_password: ""
# Destination cluster configuration. This is typically a centralized Redpanda
# cluster that aggregates the data produced by all agents.
destination:
    # Name of the destination cluster
    name: "destination"
    # List of Redpanda nodes
    bootstrap_servers: 127.0.0.1:29092
    # Maximum Kafka protocol version. E.g. "3.0.0"
    max_version: ""
    # List of topics to pull from the destination cluster (to create a
    # bidirectional flow).
    #   - Specify a single topic name to pull events to a topic with the same
    #     name on the source cluster (e.g. "configA").
    #   - Specify a pair of topic names, separated by a colon, to pull events
    #     to a topic with a different name on the source cluster
    #     (e.g. "configB:configC").
    topics:
        - configA
        - configB:configC
    # Set the consumer group for the agent to join and consume in. Defaults to
    # the agent id if not set.
    consumer_group_id: ""
    # The destination cluster TLS configuration.
    tls:
        enabled: true
        client_key: "agent.key"
        client_cert: "agent.crt"
        ca_cert: "ca.crt"
    # The destination cluster SASL configuration.
    sasl:
        # Valid SASL methods: PLAIN | SCRAM-SHA-256 | SCRAM-SHA-512
        sasl_method: ""
        sasl_username: ""
        sasl_password: ""
```
