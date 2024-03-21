#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Remove NodeGroup"
echo "####################################################################"

source ../init.sh

CLUSTERID_ADD=${OPTION03:-1}

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}${CLUSTERID_ADD}
NODEGROUPNAME="ng${INDEX}${REGION}${CLUSTERID_ADD}"

echo "NSID: "${NSID}
echo "CLUSTERID=${CLUSTERID}"
echo "NODEGROUPNAME=${NODEGROUPNAME}"

resp=$(
	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}/nodegroup/${NODEGROUPNAME}?force=true
	); echo ${resp} | jq ''
    echo ""
