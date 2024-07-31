#!/bin/bash

RED='\033[0;31m'
LGREEN='\033[1;32m'
NC='\033[0m' # No Color

CONTAINER_NAME_READ=$1
CONTAINER_VERSION=$2
CONTAINER_PORT=$3
CONTAINER_DATA_PATH=$4
CONTAINER_ENV=$5

if [ -z "$TB_ROOT_PATH" ]; then
    SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
    export TB_ROOT_PATH=`cd $SCRIPT_DIR && cd .. && pwd`
fi

CONTAINER_NAME=`echo $CONTAINER_NAME_READ | tr [:upper:] [:lower:]`
CONTAINER_VOLUME_PATH="$TB_ROOT_PATH/container-volume/${CONTAINER_NAME}-container"
CONTAINER_VOLUME_OPTION="-v $CONTAINER_VOLUME_PATH:$CONTAINER_DATA_PATH"
# If CONTAINER_DATA_PATH is not used, disable -v option 
if [ -z "$CONTAINER_DATA_PATH" ]; then
    CONTAINER_VOLUME_OPTION=""
fi

echo
echo ==========================================================
echo "[Info]"
echo ==========================================================
echo "This script runs a $CONTAINER_NAME_READ docker container."
echo "(If $CONTAINER_NAME_READ is running already, no need to continue.)"
echo
echo -e "- Container Name: ${LGREEN} $CONTAINER_NAME_READ ${NC}"
echo -e "- Container Version: ${LGREEN} $CONTAINER_VERSION ${NC} (you can change)"
echo -e "- Container Port Option: ${LGREEN} $CONTAINER_PORT ${NC}"
echo -e "- Container Volume Option: ${LGREEN} $CONTAINER_VOLUME_OPTION ${NC}"
echo -e "- Container Environment Option: ${LGREEN} $CONTAINER_ENV ${NC}"
echo

while true; do
    read -p 'Do you want to proceed ? (y/n) : ' CHECKPROCEED
    case $CHECKPROCEED in
    [Yy]*)
        break
        ;;
    [Nn]*)
        echo
        echo "Stop $0 See you soon :)"
        exit 1
        ;;
    *)
        echo "Please answer yes or no."
        ;;
    esac
done

if [ -z "$TB_ROOT_PATH" ]; then
    echo 
    echo ==========================================================
    echo "[Warning]"
    echo ==========================================================
    echo
    echo "The environment variable for \$TB_ROOT_PATH is empty."
    echo "\$TB_ROOT_PATH is the base path of persistent volume for $CONTAINER_NAME_READ."
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
    echo
    echo "Docker isn't running. Please install or start Docker!"
    echo "Installation Ref: https://docs.docker.com/engine/install/ubuntu/#installation-methods"
    echo "You can use `sudo ./scripts/runContainer.sh` script to install"
    echo
    exit 1
fi

# Stop and remove running cotainer to prevent duplication.
echo
echo ==========================================================
echo "[Stop existing $CONTAINER_NAME_READ container to prevent duplication]"
echo ==========================================================
sudo aa-remove-unknown
sudo docker stop $CONTAINER_NAME

# Run the $CONTAINER_NAME_READ container
echo
echo
echo ==========================================================
echo "[Confirm the command to run $CONTAINER_NAME_READ container]"
echo ==========================================================
echo -e "${LGREEN}"
RUNCMD="sudo docker run --rm $CONTAINER_PORT \\
                $CONTAINER_VOLUME_OPTION \\
                $CONTAINER_ENV \\
                --name $CONTAINER_NAME \\
                cloudbaristaorg/$CONTAINER_NAME:"
echo "${RUNCMD}${CONTAINER_VERSION}"
echo -e "${NC}"
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
                echo -e "${LGREEN}"
                curl -s https://registry.hub.docker.com/v2/repositories/cloudbaristaorg/$CONTAINER_NAME/tags?page_size=1024 | jq '."results"[]["name"]' | tr -d \" | sort -V
                echo -e "${NC}"
                read -p "Please specify $CONTAINER_NAME_READ version you want (latest / $CONTAINER_VERSION / ...): " CONTAINER_VERSION
                echo
                if [ "$CONTAINER_VERSION" == "latest" ]; then
                    echo "Pull the latest image from image repository"
                    sudo docker pull cloudbaristaorg/$CONTAINER_NAME
                fi 
                echo
                echo ==========================================================
                echo "[Check the command to run $CONTAINER_NAME_READ container]"
                echo ==========================================================
                echo -e "${LGREEN}"
                echo "${RUNCMD}${CONTAINER_VERSION}"
                echo -e "${NC}"
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
echo "- To get a shell from container:"
echo -e "  [${RED} sudo docker exec -it $CONTAINER_NAME /bin/bash ${NC}]"
echo
echo "- To stop container:"
echo -e "  [${RED} Ctrl+C ${NC}] or [${RED} sudo docker stop $CONTAINER_NAME ${NC}]"
echo
echo "- To delete container volume:"
echo -e "  [${RED} sudo rm -rf $CONTAINER_VOLUME_PATH ${NC}]"
echo ==========================================================
echo
sleep 3
sudo docker run --rm $CONTAINER_PORT \
    $CONTAINER_VOLUME_OPTION \
    $CONTAINER_ENV \
    --name $CONTAINER_NAME \
    cloudbaristaorg/$CONTAINER_NAME:$CONTAINER_VERSION
