#!/bin/bash

nodeId=${1:-vm}
nodeIp=${2:-127.0.0.1}
targetPort=${3:-80}

## haproxy can be replaced

echo "        server             ${nodeId} ${nodeIp}:${targetPort} check" | sudo tee -a /etc/haproxy/haproxy.cfg

## show config
cat /etc/haproxy/haproxy.cfg
