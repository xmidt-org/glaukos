# A Glaukos Cluster
In this docker-compose cluster, we will have:
* Caduceus: one server for listening for incoming events
* Caduceator: two servers, one for sending online events and one for sending offline events
* Svalinn: one server for inserting caduceus events into the codex database
* Gungnir: one server for glaukos to retrieve events related to a device id
* Glaukos
* Argus
* Prometheus

## Deploy
```
# Build glaukos image
cd ${GLAUKOS_REPO_DIR}
make docker

# Stand up docker-compose cluster
cd deploy/docker-compose
./deploy.sh
```

You can then navigate to the prometheus URL (defaults to `localhost:9090`) to see the metadata metrics that glaukos outputs.