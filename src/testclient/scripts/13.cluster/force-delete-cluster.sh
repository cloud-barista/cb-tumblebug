#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Delete"
echo "####################################################################"

source ../init.sh

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}

echo "NSID: "${NSID}
echo "CLUSTERID=${CLUSTERID}"

resp=$(
	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}?force=true
	); echo ${resp} | jq ''
    echo ""
