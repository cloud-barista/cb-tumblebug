#!/bin/bash

# Need to be executed with sudo
echo "[Install Docker]"

echo
echo "Will run the following command (the script needs to be executed with sudo)"
echo "1) sudo apt update -y"
echo "2) wget -qO- get.docker.com | sh"
echo "3) sudo docker info"

# Install Docker engine
# update
echo == apt update
sudo apt update -y

echo == install Docker engine
sleep 2
# install docker
wget -qO- get.docker.com | sh

# installation checking
echo == sudo docker info
sudo docker info
