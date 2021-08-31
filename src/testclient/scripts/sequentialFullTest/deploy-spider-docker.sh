#!/bin/bash

echo "####################################################################"
echo "## deploy-spider-docker-mcis"
echo "####################################################################"

source ../init.sh

CMD="wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/assets/scripts/setcbsp.sh -O ~/setcbtb.sh; chmod +x ~/setcbtb.sh; ~/setcbtb.sh"
echo "CMD: $CMD"

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${CMD}"
	}
EOF
)
echo "${VAR1}" | jq ''
echo ""
