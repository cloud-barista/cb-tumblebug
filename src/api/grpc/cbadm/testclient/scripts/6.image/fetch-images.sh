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

$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm image fetch --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NSID | jq ''
echo ""

source ../common-functions.sh
printElapsed $@
