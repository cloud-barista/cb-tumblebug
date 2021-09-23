#!/bin/bash

echo "####################################################################"
echo "## Deploy a Game server to MCIS "
echo "####################################################################"

source ../init.sh

CMD="wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setgame.sh -O ~/setgame.sh; chmod +x ~/setgame.sh; sudo ~/setgame.sh"
echo "CMD: $CMD"

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${CMD}"
	}
EOF
)
echo "${VAR1}" | jq ''
echo ""
