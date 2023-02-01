
import logging

from schema_registry import SchemaRegistry
from kafka import KafkaConsumer

logging.basicConfig(level=logging.INFO)

# Ref:
#   - ../docker-compose-kafka.yaml
#   - ../agent.yaml
topic_name = "configA"
cluster_hosts = "localhost:19092"
schema_registry_url = "http://localhost:18081"

registry = SchemaRegistry(schema_registry_url)

consumer = KafkaConsumer(
    bootstrap_servers=cluster_hosts,
    group_id="demo-group",
    auto_offset_reset="earliest",
    enable_auto_commit=False,
    consumer_timeout_ms = 5000
)
consumer.subscribe(topic_name)

for msg in consumer:
    value = registry.decode(msg.value)
    topic_info = f"topic: {msg.topic} ({msg.partition}|{msg.offset})"
    message_info = f"key: {msg.key}, {value}"
    logging.info(f"{topic_info}, {message_info}")
