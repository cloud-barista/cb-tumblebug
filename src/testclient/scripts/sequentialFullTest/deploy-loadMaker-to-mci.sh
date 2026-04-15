#!/bin/bash

echo "####################################################################"
echo "## deploy-loadMaker-infra "
echo "####################################################################"

source ../init.sh

CMD="wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/runLoadMaker.sh -O ~/runLoadMaker.sh; chmod +x ~/runLoadMaker.sh; sudo ~/runLoadMaker.sh"
echo "CMD: $CMD"

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/infra/$InfraID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${CMD}]"
	}
EOF
)
echo "${VAR1}" | jq '.'
echo ""
