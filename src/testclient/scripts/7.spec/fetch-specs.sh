#!/bin/bash

#function fetch_specs() {

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
echo "## 7. spec: Fetch"
echo "####################################################################"

curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/fetchSpecs | jq '' #|| return 1
#}

source ../common-functions.sh
printElapsed $@
#fetch_specs
