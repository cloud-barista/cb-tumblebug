#!/bin/bash

echo "####################################################################"
echo "## 13. Cluster: Delete"
echo "####################################################################"

source ../init.sh

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}

echo "NSID: "${NSID}
echo "CLUSTERID=${CLUSTERID}"

resp=$(
	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}
	); echo ${resp} | jq ''
    echo ""
