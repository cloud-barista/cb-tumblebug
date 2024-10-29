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
    echo "2) Use 'localhost' for Tumblebug and use 'host.docker.internal' to communicate with Spider/Dragonfly/Terrarium containers"
    read -p "Enter your choice (1 or 2): " user_choice

    # Note - EP: Endpoint
    case $user_choice in
        1)
            EP_TUMBLEBUG=$external_ip
            EP_SPIDER=$external_ip
            EP_DRAGONFLY=$external_ip
            EP_TERRARIUM=$external_ip
            ;;
        2)
            EP_TUMBLEBUG="localhost"
            EP_SPIDER="host.docker.internal"
            EP_DRAGONFLY="host.docker.internal"
            EP_TERRARIUM="host.docker.internal"            
            ;;
        *)
            echo "Invalid choice, use 'localhost' and 'host.docker.internal' as the default."
            EP_SPIDER="host.docker.internal"
            EP_DRAGONFLY="host.docker.internal"
            EP_TUMBLEBUG="localhost"
            EP_TERRARIUM="host.docker.internal"
            ;;
    esac
else
    # If external IP retrieval failed, default to localhost
    echo "Failed to retrieve external IP, use 'localhost' and 'host.docker.internal' as the default."
    EP_TUMBLEBUG="localhost"
    EP_SPIDER="host.docker.internal"
    EP_DRAGONFLY="host.docker.internal"    
    EP_TERRARIUM="host.docker.internal"
fi

echo
echo "Note - this script assumes CB-Spider/CB-Dragonfly/MC-Terrarium container is running in the same host. ($external_ip)"
echo

if [ "$user_choice" != "1" ]; then
    CONTAINER_ENV="--add-host host.docker.internal:host-gateway -e TB_SPIDER_REST_URL=http://$EP_SPIDER:1024/spider -e TB_DRAGONFLY_REST_URL=http://$EP_DRAGONFLY:9090/dragonfly -e TB_TERRARIUM_REST_URL=http://$EP_TERRARIUM:8055/terrarium -e TB_SELF_ENDPOINT=$EP_TUMBLEBUG:1323"
else
    CONTAINER_ENV="-e TB_SPIDER_REST_URL=http://$EP_SPIDER:1024/spider -e TB_DRAGONFLY_REST_URL=http://$EP_DRAGONFLY:9090/dragonfly -e TB_SELF_ENDPOINT=$EP_TUMBLEBUG:1323"    
fi

./runContainer.sh "$CONTAINER_NAME_READ" "$CONTAINER_VERSION" "$CONTAINER_PORT" "$CONTAINER_DATA_PATH" "$CONTAINER_ENV"
