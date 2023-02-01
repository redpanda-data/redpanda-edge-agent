import logging
import requests
import json

from io import BytesIO
from struct import pack, unpack
from fastavro import parse_schema
from fastavro import schemaless_reader
from fastavro import schemaless_writer

logging.basicConfig(level=logging.INFO)

class SchemaRegistry:
    """ Interact with the Schema Registry. 

    Attributes
    ----------
    base_uri : str
        The address of the Schema Registry (e.g. https://localhost:8081)
    """

    def __init__(self, uri):
        self.base_uri = uri
        self.cache = {}

    def register(self, schema):
        """ Register schema and return the schema ID.

        Parameters
        ----------
        schema : dict
            The Avro schema
        """
        subject = f"{schema['name']}-value"
        r = requests.post(
            url = f"{self.base_uri}/subjects/{subject}/versions",
            data = json.dumps({"schema": json.dumps(schema)}),
            headers={"Content-Type": "application/vnd.schemaregistry.v1+json"}
        )
        r.raise_for_status()
        id = r.json()["id"]
        logging.info(f"Registered schema id: {id}")
        return id

    def fetch(self, id):
        """ Returns the schema from the local cache,
        or fetches it from the Schema Registry.

        Parameters
        ----------
        id : int
            The ID of the schema stored in the Schema Registry
        """
        parsed_schema = self.cache.get(id, None)
        if parsed_schema:
            return parsed_schema

        r = requests.get(
            url = f"{self.base_uri}/schemas/ids/{id}"    
        )
        r.raise_for_status()
        schema = r.json()["schema"]
        parsed_schema = parse_schema(json.loads(schema))
        logging.info(f"Cached schema for id {id}: {parsed_schema}")
        self.cache[id] = parsed_schema
        return parsed_schema

    @staticmethod
    def encode(data, schema, schema_id):
        """ Encode data as Avro with Schema Registry framing:

        | Bytes | Desc                     |
        | ----- | ------------------------ |
        | 0     | Magic byte `0`           |
        | 1-4   | 4-byte schema ID (int)   |
        | 5+    | Binary encoded Avro data |

        Parameters
        ----------
        data      : dict
            The data to encode as Avro
        schema    : Schema
            The parsed schema used to encode the data
        schema_id : int
            The schema ID to encode in the Schema Registry framing
        """
        with ContextBytesIO() as bio:
            # Write the magic byte and schema ID to the buffer
            bio.write(pack(">bI", 0, schema_id))
            # Write the Avro encoded data to the buffer
            schemaless_writer(bio, schema, data)
            return bio.getvalue()

    def decode(self, data):
        """ Decode Avro data with Schema Registry framing. 

        Parameters
        ----------
        data : dict
            The Avro data to decode
        """
        with ContextBytesIO(data) as payload:
            _, msg_schema_id = unpack('>bI', payload.read(5))
            msg_schema = self.fetch(msg_schema_id)
            msg = schemaless_reader(payload, msg_schema)
            return msg

class ContextBytesIO(BytesIO):
    """ Wrapper for BytesIO for use with a context manager. """
    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()
        return False
