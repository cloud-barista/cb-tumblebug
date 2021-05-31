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

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

if [ "$CSP" == '' ]; then #|| [ "$CSP" == "all" ]
	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm image fetch --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o yaml --ns $NSID --cc "!all"

else
	source ../common-functions.sh
	getCloudIndex $CSP

    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm image fetch --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o yaml --ns $NSID --cc "${CONN_CONFIG[$INDEX,$REGION]}"

fi

source ../common-functions.sh
printElapsed $@
