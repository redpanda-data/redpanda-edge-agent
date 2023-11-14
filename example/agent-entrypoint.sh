#!/usr/bin/env bash

set -ex

# Start Redpanda
./entrypoint.sh $@ &

# Redpanda's schema registry lazily creates the _schemas topic, so poke the
# endpoint to create it before the agent starts.
sleep 10
curl -s http://localhost:8081/schemas/types

exec redpanda-edge-agent \
     -config /etc/redpanda/agent.yaml \
     -loglevel debug
