#!/bin/bash

DIR=$( cd "$(dirname "$0")" || exit; pwd -P )
ROOT_DIR=$DIR/../../

echo "Running services..."
CADUCEUS_VERSION=${CADUCEUS_VERSION:-latest} \
ARGUS_VERSION=${ARGUS_VERSION:-latest} \
GLAUKOS_VERSION=${GLAUKOS_VERSION:-latest} \
CADUCEATOR_VERSION=${CADUCEATOR_VERSION:-latest} \
SVALINN_VERSION=${SVALINN_VERSION:-0.14.0} \
GUNGNIR_VERSION=${GUNGNIR_VERSION:-0.12.3} \
docker-compose -f ./docker-compose.yml up -d $@
if [[ $? -ne 0 ]]; then
  exit 1
fi

sleep 5
AWS_ACCESS_KEY_ID=accessKey AWS_SECRET_ACCESS_KEY=secretKey aws dynamodb  --endpoint-url http://localhost:8000 describe-table --table-name gifnoc --region us-east-2 --output text > /dev/null 2> /dev/null
if [[ $? -ne 0 ]]; then
  AWS_ACCESS_KEY_ID=accessKey AWS_SECRET_ACCESS_KEY=secretKey aws dynamodb  --endpoint-url http://localhost:8000 create-table \
      --table-name gifnoc \
      --attribute-definitions \
          AttributeName=bucket,AttributeType=S \
          AttributeName=id,AttributeType=S \
      --key-schema \
          AttributeName=bucket,KeyType=HASH \
          AttributeName=id,KeyType=RANGE \
      --provisioned-throughput \
          ReadCapacityUnits=10,WriteCapacityUnits=5 \
      --stream-specification StreamEnabled=true,StreamViewType=NEW_AND_OLD_IMAGES \
      --region us-east-2 \
      --output text

  AWS_ACCESS_KEY_ID=accessKey AWS_SECRET_ACCESS_KEY=secretKey aws dynamodb \
    --endpoint-url http://localhost:8000 --region us-east-2 update-time-to-live \
    --table-name gifnoc --time-to-live-specification "Enabled=true, AttributeName=expires" \
    --output text
fi

docker exec -it yb-tserver-n1 /home/yugabyte/bin/cqlsh yb-tserver-n1 -f /create_db.cql
