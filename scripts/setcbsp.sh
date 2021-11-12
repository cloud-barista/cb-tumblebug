#!/bin/bash
sudo apt-get update > /dev/null
sudo apt-get -y install apt-transport-https ca-certificates curl gnupg-agent software-properties-common > /dev/null
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo apt-key fingerprint 0EBFCD88
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
sudo apt-get update > /dev/null
sudo apt-get -y install docker-ce docker-ce-cli containerd.io > /dev/null
sudo docker run --rm -p 1024:1024 -p 2048:2048 -v /root/go/src/github.com/cloud-barista/cb-spider/meta_db:/root/go/src/github.com/cloud-barista/cb-spider/meta_db --name cb-spider cloudbaristaorg/cb-spider:0.4.18

# Print a message (IP address)
str=$(curl https://api.ipify.org)
printf "CB-Spider is ready. Access to http://%s:1024/spider/ ." $str