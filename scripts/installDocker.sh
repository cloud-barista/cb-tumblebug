#!/bin/bash

# Need to be executed with sudo
echo "[Install Docker]"

echo
echo "Will run the following command (the script needs to be executed with sudo)"
echo "1) sudo apt update -y"
echo "2) wget -qO- get.docker.com | sh"
echo "3) sudo docker -v"

# Check Docker engine
if ! sudo docker -v 2>&1; then
    # update
    echo "[apt update]"
    sudo apt update -y

    # install Docker
    echo "[install Docker engine]"
    wget -qO- get.docker.com | sh
fi

# installation checking
echo [sudo docker -v]
sudo docker -v
