#!/bin/bash

source ../init.sh

echo "####################################################################"
echo "## Install monitoring agent to MCI "
echo "####################################################################"

curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/monitoring/install/mci/$MCIID -H 'Content-Type: application/json' -d \
	'{
			"command": "echo -n [CMD] Works! [Hostname: ; hostname ; echo -n ]"
	}' | jq ''
