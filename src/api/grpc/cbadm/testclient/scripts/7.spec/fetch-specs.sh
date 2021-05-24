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

$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm spec fetch --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NSID | jq '' #|| return 1
#}

source ../common-functions.sh
printElapsed $@
#fetch_specs
