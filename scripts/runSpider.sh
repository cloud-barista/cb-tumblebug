#!/bin/bash

CONTAINER_NAME_READ="CB-Spider"
CONTAINER_VERSION="0.6.17"
CONTAINER_PORT="-p 1024:1024 -p 2048:2048"
CONTAINER_DATA_PATH="/root/go/src/github.com/cloud-barista/cb-spider/meta_db"

parent_path=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
cd "$parent_path"

./runContainer.sh "$CONTAINER_NAME_READ" "$CONTAINER_VERSION" "$CONTAINER_PORT" "$CONTAINER_DATA_PATH"
