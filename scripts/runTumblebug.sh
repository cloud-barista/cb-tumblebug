#!/bin/bash

CONTAINER_NAME_READ="CB-Tumblebug"
CONTAINER_VERSION="latest"
CONTAINER_PORT="-p 1323:1323"
CONTAINER_DATA_PATH="/app/meta_db"

parent_path=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
cd "$parent_path"

# Get IP address which is accessable from outsite.
# `https://api.ipify.org` is one of IP lookup services. If it is not available we need to change.
echo "[Retrieve IP address accessible from outside]"
external_ip=$(curl -s https://api.ipify.org)

if [[ -n "$external_ip" ]]; then
    # If external IP retrieval was successful, prompt user to select the ENDPOINT
    echo "Please select endpoints to be used:"
    echo "1) Use External IP for all components: $external_ip"
    echo "2) Use 'host.docker.internal' to communicate with Spider and Dragonfly containers, 'localhost' for Tumblebug"
    read -p "Enter your choice (1 or 2): " user_choice


    case $user_choice in
        1)
            SP_ENDPOINT=$external_ip
            DF_ENDPOINT=$external_ip
            TB_ENDPOINT=$external_ip
            ;;
        2)
            SP_ENDPOINT="host.docker.internal"
            DF_ENDPOINT="host.docker.internal"
            TB_ENDPOINT="localhost"
            ;;
        *)
            echo "Invalid choice, use 'host.docker.internal' and 'localhost' as the default."
            SP_ENDPOINT="host.docker.internal"
            DF_ENDPOINT="host.docker.internal"
            TB_ENDPOINT="localhost"
            ;;
    esac
else
    # If external IP retrieval failed, default to localhost
    echo "Failed to retrieve external IP, use 'host.docker.internal' and 'localhost' as the default."
    SP_ENDPOINT="host.docker.internal"
    DF_ENDPOINT="host.docker.internal"
    TB_ENDPOINT="localhost"
fi

echo
echo "This script assume CB-Spider container is running in the same host. ($external_ip)"
echo

if [ "$user_choice" != "1" ]; then
    CONTAINER_ENV="--add-host host.docker.internal:host-gateway -e SPIDER_REST_URL=http://$SP_ENDPOINT:1024/spider -e DRAGONFLY_REST_URL=http://$DF_ENDPOINT:9090/dragonfly -e SELF_ENDPOINT=$TB_ENDPOINT:1323"
else
    CONTAINER_ENV="-e SPIDER_REST_URL=http://$SP_ENDPOINT:1024/spider -e DRAGONFLY_REST_URL=http://$DF_ENDPOINT:9090/dragonfly -e SELF_ENDPOINT=$TB_ENDPOINT:1323"    
fi

./runContainer.sh "$CONTAINER_NAME_READ" "$CONTAINER_VERSION" "$CONTAINER_PORT" "$CONTAINER_DATA_PATH" "$CONTAINER_ENV"
