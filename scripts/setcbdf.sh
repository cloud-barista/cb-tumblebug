#!/bin/bash

sudo apt-get update > /dev/null

# Install GIT
sudo apt-get -y install git > /dev/null

# Install Go
wget https://dl.google.com/go/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz

# Set Go env (for next interactive shell)
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc 
echo  'export GOPATH=$HOME/go' >> ~/.bashrc 
# Set Go env (for current shell)
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go

# Install some dependencies
sudo apt-get -y install make > /dev/null
sudo apt-get -y install gcc > /dev/null
sudo apt-get -y install docker-compose > /dev/null

# Download cb-operator repo with dependencies
export GO111MODULE=off
go get -v github.com/cloud-barista/cb-operator 

# Set cb-operator configuration
printf DockerCompose > $GOPATH/src/github.com/cloud-barista/cb-operator/src/CB_OPERATOR_MODE

# Set cb-tumblebug environment for go module
export GO111MODULE=on

# Build tumblebug
cd ~/go/src/github.com/cloud-barista/cb-operator/src/
make

# Print a message (IP address)
str=$(curl https://api.ipify.org)
printf "cb-operator is ready. Access to it with ssh %s " $str

sudo sed -i "s/127.0.0.1/`curl https://api.ipify.org`/g" $GOPATH/src/github.com/cloud-barista/cb-operator/docker-compose-mode-files/conf/cb-dragonfly/config.yaml

nohup sudo $GOPATH/src/github.com/cloud-barista/cb-operator/src/operator run -f $GOPATH/src/github.com/cloud-barista/cb-operator/docker-compose-mode-files/docker-compose-df-only.yaml 1>/dev/null 2>&1 &
# $GOPATH/src/github.com/cloud-barista/cb-operator/src/operator info -f $GOPATH/src/github.com/cloud-barista/cb-operator/docker-compose-mode-files/docker-compose-df-only.yaml
printf "cb-dragonfly is ready. Access to it with ssh %s " $str
