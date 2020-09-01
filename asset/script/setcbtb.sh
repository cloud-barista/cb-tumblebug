#!/bin/bash

sudo apt-get update > /dev/null

# Install GIT
sudo apt-get -y install git > /dev/null

# Install Go
wget https://dl.google.com/go/go1.13.4.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.13.4.linux-amd64.tar.gz

# Set Go env (for next interactive shell)
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc 
echo  'export GOPATH=$HOME/go' >> ~/.bashrc 
# Set Go env (for current shell)
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go

# Install some dependencies
sudo apt-get -y install make > /dev/null
sudo apt-get -y install gcc > /dev/null

# Download cb-tumblebug repo with dependencies
export GO111MODULE=off
go get -v github.com/cloud-barista/cb-tumblebug 

# Set cb-tumblebug configuration
echo  'source $GOPATH/src/github.com/cloud-barista/cb-tumblebug/conf/setup.env' >> ~/.bashrc 
source $GOPATH/src/github.com/cloud-barista/cb-tumblebug/conf/setup.env

# Set cb-tumblebug environment for go module
export GO111MODULE=on

# Build tumblebug
cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src/
make

# Print a message (IP address)
str=$(curl https://api.ipify.org)
printf "CB-Tumblebug is ready. Access to it with ssh %s \n[Execute] source cb-tumblebug/conf/setup.env" $str


