#!/bin/bash

echo "####################################################################"
echo "## deploy-nginx-mci "
echo "####################################################################"

source ../init.sh

CMD="wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setweb.sh -O ~/setweb.sh; chmod +x ~/setweb.sh; sudo ~/setweb.sh"
echo "CMD: $CMD"

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mci/$MCIID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${CMD}]"
	}
EOF
)
echo "${VAR1}" | jq ''
echo ""
