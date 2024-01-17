#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Remove NodeGroup"
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
	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}/nodegroup/${NODEGROUPNAME}?force=true
	); echo ${resp} | jq ''
    echo ""
