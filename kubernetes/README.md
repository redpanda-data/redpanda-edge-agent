# Edge Agent Kubernetes Example

The [deploy](deploy.yaml) file will deploy a single-replica statefulset with a container running the redpanda edge agent. A configmap is included in this file, and will need to be modified for your deployment.

The instructions below assume you don't already have source and destination clusters available, and will walk you through the steps for both deploying these two clusters and deploying the redpanda edge agent. If you already have source and destination clusters, then you can skip [the steps](#deploy-source-and-destinaton-clusters) for deploying these clusters.

## Prerequisites

An edge agent image must be built and pushed to an external registry in order to be available from within Kubernetes. The instructions below assume you are using Docker Hub.

Clone this repository (if you haven't already):

```bash
git clone https://github.com/redpanda/redpanda-edge-agent
cd redpanda-edge-agent
```

There are two build scripts in this repo: one for [the agent](../build.sh) and another for [the image](../docker/build.sh). We only need to run the image build script, since the agent will be built as a step within the image.

Build the image, passing in the Docker username that will be used when both naming the image and pushing the image to Docker Hub. Replace `redpanda` below with your username:
```bash
./docker/build.sh -u redpanda
```

Credentials for the command above are pulled from the credential store that Docker is configured to use. For more details see the following links:
- https://github.com/docker/docker-credential-helpers
- https://steinbaugh.com/posts/docker-credential-pass.html
- https://github.com/docker/docker-credential-helpers/issues/102

The image is now available at `$USERNAME/redpanda-edge-agent` with a default tag of `latest`. To customize the tag or otherwise modify how the build script works, run `./docker/build.sh -h`.

## Deploy source and destinaton clusters

Use the helm chart to deploy two single-node clusters. Both will be deployed into the same namespace with external access disabled in the steps below. 

The helm commands below assume you already have an available Kubernetes environment defined. If you don't the instructions [here](https://docs.redpanda.com/docs/platform/quickstart/kubernetes-qs-dev/#create-a-kubernetes-cluster).

Deploy the source cluster:
```bash
helm upgrade --install redpanda-src redpanda/redpanda -n redpanda --create-namespace --set "statefulset.replicas=1" --set "external.enabled=false"
```

Now deploy the destination cluster:
```bash
helm upgrade --install redpanda-dest redpanda/redpanda -n redpanda --create-namespace --set "statefulset.replicas=1" --set "external.enabled=false"
```

There are several ways to check status of these deployments. One way is to view the status of the deployed resources with the following command:

```bash
watch -n 1 kubectl get all -A -o wide --field-selector=metadata.namespace=redpanda
```

## Deploy the agent

Now you can deploy the agent. There are several defaults to note:
- the agent is deployed within the `redpanda` namespace
- the source cluster is available from `redpanda-src:10092`
- the destination cluster is available from `redpanda-dest:10092`

These and other defaults can be changed by editing the ConfigMap resource within [deploy.yaml](deploy.yaml).

Once you have edited the ConfigMap resource so that it is appropriate for your environment, you can deploy the agent:

```bash
kubectl apply -f kubernetes/deploy.yaml
```

You can monitor the progress of this deployment with the `watch...` command above, and also by monitoring events for the appropriate namespace (by default `redpanda`):

```bash
kubectl get events -w -n redpanda
```


## Test the agent

Produce some messages to the source's `telemetry1` topic (note that the [ConfigMap](deploy.yaml) is configured to create the topics on startup):

```bash
kubectl exec -it -n redpanda redpanda-src-0 -c redpanda -- /bin/bash
for i in {1..60}; do echo $(cat /dev/urandom | head -c10 | base64) | rpk topic produce telemetry1; sleep 1; done
```

The agent will forward the messages to a topic with the same name on the destination. Open a second terminal and consume the messages:

```bash
kubectl exec -it -n redpanda redpanda-dest-0 -c redpanda -- /bin/bash
rpk topic consume telemetry1
{
  "topic": "telemetry1",
  "key": "51940184cb08",
  "value": "q/F5LP5DmnIPog==",
  "timestamp": 1667984441252,
  "partition": 0,
  "offset": 0
}
{
  "topic": "telemetry1",
  "key": "51940184cb08",
  "value": "5ATcnSvzmd3vOw==",
  "timestamp": 1667984442624,
  "partition": 0,
  "offset": 1
}
{
  "topic": "telemetry1",
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
kubectl logs redpanda-edge-agent-0 -n redpanda -f

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
