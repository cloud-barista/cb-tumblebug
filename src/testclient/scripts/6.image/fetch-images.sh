#!/bin/bash

SECONDS=0

TestSetFile=${4:-../testSet.env}

FILE=$TestSetFile
if [ ! -f "$FILE" ]; then
    echo "$FILE does not exist."
    exit
fi
source $TestSetFile
source ../conf.env

echo "####################################################################"
echo "## 6. image: Fetch"
echo "####################################################################"

# curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/fetchImages | jq ''
# echo ""

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

if [ "$CSP" == '' ]; then #|| [ "$CSP" == "all" ]
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/fetchImages | jq '' #|| return 1

else
	source ../common-functions.sh
	getCloudIndex $CSP


	resp=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/fetchImages -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}"
			}
EOF
	); echo ${resp} | jq ''
	echo ""

fi

source ../common-functions.sh
printElapsed $@
