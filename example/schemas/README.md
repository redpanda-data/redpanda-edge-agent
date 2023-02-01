# Schemas Example

How to synchronize the schema registry topic using the Edge Agent. This is particularly useful for pulling centrally managed schemas out to edge environments for use locally.

## Pull schemas topic

Add the `_schemas` topic to the destination clusters topic list. E.g. see [agent.yaml](../agent.yaml).

## Start the containers

```bash
cd example
docker-compose -f docker-compose-kafka.yaml up -d
[+] Running 5/5
 ⠿ Network example_redpanda_network  Created
 ⠿ Container zookeeper               Started
 ⠿ Container redpanda_source         Started
 ⠿ Container kafka_destination       Started
 ⠿ Container schema_registry         Started
```

## Create Python environment

```bash
cd example/schemas
python3 -m venv env
source env/bin/activate
pip install --upgrade pip
pip install -r requirements.txt
```

## Produce Avro records on destination

```bash
export REDPANDA_BROKERS=localhost:29092
python produce_avro.py

INFO:root:Sent to topic 'configA' at offset 0: {"epoch": 1675241406407434000, "uuid": "5a4e3c5494a04338866fafd4a9789aaf"}
INFO:root:Sent to topic 'configA' at offset 1: {"epoch": 1675241407436049000, "uuid": "60d9d85c9ef04ff0952c432bf7e3ba01"}
INFO:root:Sent to topic 'configA' at offset 2: {"epoch": 1675241408441169000, "uuid": "264a0a38698042f2b774cb191c95399b"}
...
```

## Check that the schema has been synced

```bash
export REDPANDA_BROKERS=localhost:19092
rpk topic consume _schemas

{
  "topic": "_schemas",
  "key": "{\"keytype\":\"SCHEMA\",\"subject\":\"demo-value\",\"version\":1,\"magic\":1}",
  "value": "{\"subject\":\"demo-value\",\"version\":1,\"id\":1,\"schema\":\"{\\\"type\\\":\\\"record\\\",\\\"name\\\":\\\"demo\\\",\\\"fields\\\":[{\\\"name\\\":\\\"epoch\\\",\\\"type\\\":\\\"int\\\"},{\\\"name\\\":\\\"uuid\\\",\\\"type\\\":\\\"string\\\"}]}\",\"deleted\":false}",
  "timestamp": 1675241406264,
  "partition": 0,
  "offset": 2
}
```

## Consume Avro records on source

```bash
export REDPANDA_BROKERS=localhost:19092
python consume_avro.py

INFO:root:topic: configA (0|0), key: b'50c99eefd234', {'epoch': 1675241406407434000, 'uuid': '5a4e3c5494a04338866fafd4a9789aaf'}
INFO:root:topic: configA (0|1), key: b'50c99eefd234', {'epoch': 1675241407436049000, 'uuid': '60d9d85c9ef04ff0952c432bf7e3ba01'}
INFO:root:topic: configA (0|2), key: b'50c99eefd234', {'epoch': 1675241408441169000, 'uuid': '264a0a38698042f2b774cb191c95399b'}
...
```
