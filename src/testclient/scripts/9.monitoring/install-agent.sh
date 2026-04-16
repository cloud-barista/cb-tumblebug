#!/bin/bash

source ../init.sh

echo "####################################################################"
echo "## Install monitoring agent to Infra "
echo "####################################################################"

curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/monitoring/install/infra/$InfraID -H 'Content-Type: application/json' -d \
	'{
			"command": "echo -n [CMD] Works! [Hostname: ; hostname ; echo -n ]"
	}' | jq '.'
