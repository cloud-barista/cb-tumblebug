#!/bin/bash

CONTAINER_NAME_READ="CB-Tumblebug"
CONTAINER_VERSION="latest"
CONTAINER_PORT="-p 1323:1323 -p 50252:50252"
CONTAINER_DATA_PATH="/app/meta_db"

parent_path=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
cd "$parent_path"

# Get IP address which is accessable from outsite.
# `https://api.ipify.org` is one of IP lookup services. If it is not available we need to change.
echo "[Retrieve IP address that accessable from outside]"
echo
str=$(curl https://api.ipify.org)
if [ -z "$str" ]
then
    echo "The result for IP lookup is empty."
    echo "Set ENDPOINT=localhost"
    str=localhost
fi
ENDPOINT=$str
echo
echo "This script assume CB-Spider container is running in the same host. ($ENDPOINT)"
echo
CONTAINER_ENV="-e SPIDER_REST_URL=http://$ENDPOINT:1024/spider -e DRAGONFLY_REST_URL=http://$ENDPOINT:9090/dragonfly -e SELF_ENDPOINT=$ENDPOINT:1323"

./runContainer.sh "$CONTAINER_NAME_READ" "$CONTAINER_VERSION" "$CONTAINER_PORT" "$CONTAINER_DATA_PATH" "$CONTAINER_ENV"

