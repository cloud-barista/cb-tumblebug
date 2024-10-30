#!/bin/bash

source ../init.sh

echo "####################################################################"
echo "## Get MCI monitoring data (parameter: -x [cpu/memory/disk/network])"
echo "####################################################################"

USERCMD=${OPTION01}

if [ -z "$USERCMD" ]; then
	echo "[Warning] Provide monitoring metric to (-x parameter)"
	echo "Available metric: cpu | cpufreq | memory | disk | network"
	exit
fi

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/monitoring/mci/$MCIID/metric/$USERCMD | jq '.'

