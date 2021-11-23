#!/bin/bash

source ../init.sh

echo "####################################################################"
echo "## Install monitoring agent to MCIS "
echo "####################################################################"

curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/monitoring/install/mcis/$MCISID -H 'Content-Type: application/json' -d \
	'{
			"command": "echo -n [CMD] Works! [Hostname: ; hostname ; echo -n ]"
	}' | jq ''
