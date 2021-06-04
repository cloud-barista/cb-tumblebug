#!/bin/bash

#function terminate_and_delete_mcis() {

TestSetFile=${4:-../testSet.env}
if [ ! -f "$TestSetFile" ]; then
	echo "$TestSetFile does not exist."
	exit
fi
source $TestSetFile
source ../conf.env

echo "####################################################################"
echo "## 8. VM: Terminate and Delete MCIS"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}


source ../common-functions.sh
getCloudIndex $CSP



MCISID=TBD
if [ "${INDEX}" == "0" ]; then
	MCISID=${MCISPREFIX}-${POSTFIX}
else
	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${INDEX} ${REGION} ${MCISID}"


echo ""
echo "Terminate and Delete [MCIS: $MCISID]"
$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis delete --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NSID --mcis ${MCISID} | jq ''

#}

#terminate_and_delete_mcis
