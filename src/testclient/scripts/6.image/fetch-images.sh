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

curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/fetchImages | jq ''
echo ""

source ../common-functions.sh
printElapsed $@
