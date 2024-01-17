#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Set Autoscaling"
echo "####################################################################"

source ../init.sh

if [ "$CSP" == "azure" ]; then
	NODEGROUPNAME="new${INDEX}${REGION}"
else
	#NODEGROUPNAME="${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}"
	NODEGROUPNAME="new${INDEX}${REGION}"
fi 

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}

echo "NSID: "${NSID}
echo "CLUSTERID=${CLUSTERID}"
echo "NODEGROUPNAME=${NODEGROUPNAME}"

resp=$(
	curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}/nodegroup/${NODEGROUPNAME}/onautoscaling -H 'Content-Type: application/json' -d @- <<EOF
	{
		"onAutoScaling": "false"
	}
EOF
    ); echo ${resp} | jq ''
    echo ""

