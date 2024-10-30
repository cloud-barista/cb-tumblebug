#!/bin/bash

echo "####################################################################"
echo "## deploy-tumblebug-mci (source build) "
echo "####################################################################"

source ../init.sh

CMD="wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setcbtb.sh -O ~/setcbtb.sh; chmod +x ~/setcbtb.sh; ~/setcbtb.sh"
echo "CMD: $CMD"

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mci/$MCIID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${CMD}]"
	}
EOF
)
echo "${VAR1}" | jq '.'
echo ""
