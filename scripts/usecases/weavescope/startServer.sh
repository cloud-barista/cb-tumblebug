#!/bin/bash


echo "[Start Weave Scope Cluster Monitoring]"

SECONDS=0

PublicIPs=${1}
PrivateIPs=${2}

echo "Installing Weavescope to MCIS..."

ScopeInstallFile="git.io/scope"
ScopeInstallFile="https://gist.githubusercontent.com/seokho-son/bb2703ca49555f9afe0d0097894c74fa/raw/9eb65b296b85bc53f53af3e8733603d807fb9287/scope"
sudo curl -L $ScopeInstallFile -o /usr/local/bin/scope
sudo chmod a+x /usr/local/bin/scope

FILE="/usr/local/bin/scope"

echo "Installing prerequisite"
sudo apt-get update > /dev/null
sudo apt install docker.io -y

PID=$(ps -ef | grep scope | awk '{print $2}')

if [ "${PID}" != "" ]; then
        echo "scope ${PID} exist. sudo scope stop"
        sudo scope stop
fi

echo "Launching Weavescope"

sudo scope launch $PublicIPs $PrivateIPs

echo "Done! elapsed time: $SECONDS"

IP=$(curl https://api.ipify.org)

PID=$(ps -ef | grep scope | awk '{print $2}')


echo "[Start Scope: complete] PID=$PID"
echo "$IP:4040/#!/state/{\"contrastMode\":true,\"topologyId\":\"containers-by-hostname\"}"
