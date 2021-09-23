#!/bin/bash

CONTAINER_NAME_READ=$1
CONTAINER_VERSION=$2
CONTAINER_PORT=$3
CONTAINER_DATA_PATH=$4
CONTAINER_NAME=`echo $CONTAINER_NAME_READ | tr [:upper:] [:lower:]`
CONTAINER_VOLUME_PATH="$CBTUMBLEBUG_ROOT/meta_db/${CONTAINER_NAME}-container"

echo
echo ==========================================================
echo "[Info]"
echo ==========================================================
echo "This script runs a $CONTAINER_NAME_READ docker container."
echo "(If $CONTAINER_NAME_READ is running already, no need to continue.)"
echo
echo "- Container Name: $CONTAINER_NAME_READ"
echo "- Container Version: $CONTAINER_VERSION"
echo "- Container Port Option: $CONTAINER_PORT"
echo "- Container Data Path: $CONTAINER_DATA_PATH"
echo

while true; do
    read -p 'Do you want to proceed ? (y/n) : ' CHECKPROCEED
    case $CHECKPROCEED in
    [Yy]*)
        break
        ;;
    [Nn]*)
        echo
        echo "[Command: $0 $@] has been cancelled. See you soon. :)"
        exit 1
        ;;
    *)
        echo "Please answer yes or no."
        ;;
    esac
done

if [ -z "$CBTUMBLEBUG_ROOT" ]; then
    echo 
    echo ==========================================================
    echo "[Warning]"
    echo ==========================================================
    echo
    echo "The environment variable for \$CBTUMBLEBUG_ROOT is empty."
    echo "\$CBTUMBLEBUG_ROOT is the base path of persistent volume for $CONTAINER_NAME_READ."
    echo "(to show persistent volumes of the containers related to CB-Tumblebug in CB-Tumblebug directory)"
    echo
    echo "You need to execute [source conf/setup.env] before run $CONTAINER_NAME_READ container."
    exit 1
fi

echo 
echo
echo ==========================================================
echo "[Check docker is running]"
echo ==========================================================
if ! sudo docker -v 2>&1; then
    echo "Docker isn't running. Please install or start Docker!"
    echo "Check https://github.com/cloud-barista/cb-coffeehouse/tree/main/scripts/docker-setup"
    exit 1
fi

# Stop and remove running cotainer to prevent duplication.
echo
echo "[Stop existing $CONTAINER_NAME_READ container to prevent duplication]"
echo ==========================================================
sudo docker stop $CONTAINER_NAME

# Run the $CONTAINER_NAME_READ container
echo
echo
echo ==========================================================
echo "[Check the command to run $CONTAINER_NAME_READ container]"
echo ==========================================================
echo
RUNCMD="sudo docker run --rm $CONTAINER_PORT \\
                -v $CONTAINER_VOLUME_PATH:$CONTAINER_DATA_PATH \\
                --name $CONTAINER_NAME \\
                cloudbaristaorg/$CONTAINER_NAME:"
echo "${RUNCMD}${CONTAINER_VERSION}"
echo
echo

echo ==========================================================
while true; do
    read -p 'Do you want to proceed ? (y/n) : ' CHECKPROCEED
    case $CHECKPROCEED in
    [Yy]*)
        break
        ;;
    [Nn]*)
        while true; do
            read -p "Do you want to run another version of $CONTAINER_NAME_READ ? (y/n) : " CHECKPROCEED2
            case $CHECKPROCEED2 in
            [Yy]*)
                echo 
                echo ==========================================================
                echo "[List of all version tags in hub.docker]"
                echo
                curl -s https://registry.hub.docker.com/v1/repositories/cloudbaristaorg/$CONTAINER_NAME/tags | \
                        grep -oP '(?<="name": ")[^"]+' | sort -r
                read -p "Please specify $CONTAINER_NAME_READ version you want (latest / $CONTAINER_VERSION / ...): " CONTAINER_VERSION
                echo
                echo
                echo ==========================================================
                echo "[Check the command to run $CONTAINER_NAME_READ container]"
                echo ==========================================================
                echo
                echo "${RUNCMD}${CONTAINER_VERSION}"
                echo
                break
                ;;
            [Nn]*)
                echo
                echo "[Command: $0 $@] has been cancelled. See you soon. :)"
                exit 1
                ;;
            *)
                echo "Please answer yes or no."
                ;;
            esac
        done
        ;;
    *)
        echo "Please answer yes or no."
        ;;
    esac
done
echo

echo
echo ==========================================================
echo "- To stop container:"
echo "  [Ctrl+C] or [sudo docker stop $CONTAINER_NAME]"
echo
echo "- To delete container volume:"
echo "  [sudo rm -rf $CONTAINER_VOLUME_PATH]"
echo ==========================================================
echo
sleep 2
sudo docker run --rm $CONTAINER_PORT \
    -v $CONTAINER_VOLUME_PATH:$CONTAINER_DATA_PATH \
    --name $CONTAINER_NAME \
    cloudbaristaorg/$CONTAINER_NAME:$CONTAINER_VERSION
