
import logging
import uuid
import time
import json

from schema_registry import SchemaRegistry
from kafka import KafkaProducer
from kafka.errors import KafkaError
from fastavro import parse_schema

logging.basicConfig(level=logging.INFO)

# Ref:
#   - ../docker-compose-kafka.yaml
#   - ../agent.yaml
topic_name = "configA"
cluster_hosts = "localhost:29092"
schema_registry_url = "http://localhost:28081"

demo_schema = {
    "type": "record",
    "name": "demo",
    "fields": [
        {"name": "epoch", "type": "int"},
        {"name": "uuid", "type": "string"}
    ]
}

registry = SchemaRegistry(schema_registry_url)
schema_id = registry.register(demo_schema)

producer = KafkaProducer(bootstrap_servers=cluster_hosts)
parsed_schema = parse_schema(demo_schema)

for i in range(10):
    msg = {
        "epoch": time.time_ns(),
        "uuid": uuid.uuid4().hex
    }
    msg_avro = SchemaRegistry.encode(msg, parsed_schema, schema_id)
    future = producer.send(topic_name, value=msg_avro)
    try:
        meta = future.get(timeout=2)
        logging.info("Sent to topic '{}' at offset {}: {}".format(
            meta.topic, meta.offset, json.dumps(msg)))
    except KafkaError as e:
        logging.error(f"Error sending message: {e}")
    time.sleep(1)

producer.flush()
producer.close()
