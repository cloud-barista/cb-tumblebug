#!/bin/bash

mcisId=${1:-mcis}
listenMode=${2:-tcp}
listenPort=${3:-80}
balanceAlgo=${4:-roundrobin}

## haproxy can be replaced
sudo apt update > /dev/null
sudo apt install haproxy -y > /dev/null
sudo systemctl enable haproxy
haproxy -v

## show config
cat /etc/haproxy/haproxy.cfg

echo "
## define admin page for statistics dashboard
listen admin
        bind *:9000
        mode http
        stats enable
        stats refresh 10s
        stats uri /
        stats auth  default:default" | sudo tee -a /etc/haproxy/haproxy.cfg

echo "
## define frontend
frontend ${mcisId}.frontend
        bind *:$listenPort
        mode $listenMode
        default_backend    ${mcisId}.backend
        option             forwardfor" | sudo tee -a /etc/haproxy/haproxy.cfg

echo "
## define backend
backend ${mcisId}.backend
        balance            ${balanceAlgo}" | sudo tee -a /etc/haproxy/haproxy.cfg

## show config
cat /etc/haproxy/haproxy.cfg
