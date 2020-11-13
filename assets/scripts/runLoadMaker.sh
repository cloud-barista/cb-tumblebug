#!/bin/bash
sudo apt-get update > /dev/null
sudo apt-get -y install apt-transport-https ca-certificates curl gnupg-agent software-properties-common > /dev/null
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo apt-key fingerprint 0EBFCD88
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
sudo apt-get update > /dev/null
sudo apt-get -y install docker-ce docker-ce-cli containerd.io > /dev/null
sudo docker run -d -p 8888:80 -p 4433:443 jihoons/docker-nginx-systemcall:latest

# Print a message (IP address)
str=$(curl https://api.ipify.org)
printf "LoadMaker is ready. Access to http://%s:8080/ ." $str
