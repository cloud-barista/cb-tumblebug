#!/bin/bash


TestSetFile=${4:-../testSet.env}
if [ ! -f "$TestSetFile" ]; then
	echo "$TestSetFile does not exist."
	exit
fi
source $TestSetFile
source ../conf.env

echo "####################################################################"
echo "## 8. VM: List ID"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP


if [ "${INDEX}" == "0" ]; then
	MCISID=${MCISPREFIX}-${POSTFIX}
else
	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCISID}"

$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis list-vm-id --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NSID --mcis ${MCISID} | jq ''


#get_mcis
