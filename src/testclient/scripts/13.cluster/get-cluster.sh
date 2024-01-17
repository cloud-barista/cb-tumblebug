#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Get"
echo "####################################################################"

source ../init.sh

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}

echo "NSID: "${NSID}
echo "CLUSTERID=${CLUSTERID}"

resp=$(
	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}
	); echo ${resp} | jq ''
    echo ""
