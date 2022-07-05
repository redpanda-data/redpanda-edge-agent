# Redpanda Edge Agent

A lightweight internet of things (IoT) agent that runs alongside Redpanda at the edge to forward events to a central Kafka API compatible cluster. The agent is written in Go and uses the [franz-go](https://github.com/twmb/franz-go) Kafka client library.

# Install

```
go clean
go build -o redpanda-edge-agent
sudo mkdir -p ${HOME}/rpk-plugin/bin
sudo cp redpanda-edge-agent ${HOME}/rpk-plugin/bin
export PATH=${HOME}/rpk-plugin/bin:${PATH}
```

# Running the agent

The recommended way to run the agent is via Redpanda's command line utility `rpk`, but you can run it independently using Go:

```
go run main.go -loglevel=DEBUG
```

Or by running the binary:
```
Usage of redpanda-edge-agent:
  -config string
    	path to agent config file (default "agent.yaml")
  -enablelog
    	log to a file instead of stderr
  -logfile string
    	if 'enablelog' is true, then log to this file (default "${HOME}/rpk-plugin/bin/agent.log")
  -loglevel string
    	logging level (default "info")

redpanda-edge-agent -config /etc/redpanda/conf/agent.yaml -loglevel=DEBUG
```

# Configuration

Example `agent.yaml`:

```yaml
# The unique identifier for the agent. If not specified the id defaults to the hostname 
# reported by the kernel. When forwarding a record, if its key is empty, then it is set
# to the agent id to route all of the records forwarded by the same agent to the same 
# destination topic partition.
id:
# If the given topics do not exist, attempt to create them on the source and destination
# clusters.
create_topics: true
# Source cluster configuration.
source:
    bootstrap_servers: 127.0.0.1:19092
    # The list of topics to forward to the destination cluster.
    topics:
        - test
    # Set the consumer group for the agent to join and consume in.
    # Defaults to the agent id if not set.
    consumer_group_id: ""
    # If "create_topics" is true, then attempt to create the topics on the source cluster
    # with this number of partitions and replication factor.
    default_partitions: 1
    default_replication: 1
    # The source cluster TLS configuration.
    tls:
        enabled: true
        client_key: "agent.key"
        client_cert: "agent.crt"
        ca_cert: "ca.crt"
    # The source cluster SASL configuration.
    sasl:
        # Valid SASL mechanisms: plain | scram-sha-256 | scram-sha-512 | aws-msk-iam
        sasl_method: ""
        sasl_username: ""
        sasl_password: ""
# Destination cluster configuration.
destination:
    bootstrap_servers: 127.0.0.1:29092
    # If "create_topics" is true, then attempt to create the topics on the destination
    # cluster with this number of partitions and replication factor.
    default_partitions: 1
    default_replication: 1
    # The destination cluster TLS configuration.
    tls:
        enabled: true
        client_key: "agent.key"
        client_cert: "agent.crt"
        ca_cert: "ca.crt"
    # The destination cluster SASL configuration.
    sasl:
        # Valid SASL mechanisms: plain | scram-sha-256 | scram-sha-512 | aws-msk-iam
        sasl_method: ""
        sasl_username: ""
        sasl_password: ""
```
