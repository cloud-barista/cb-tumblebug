#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Delete"
echo "####################################################################"

source ../init.sh

CLUSTERID_ADD=${OPTION03:-1}

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}${CLUSTERID_ADD}

echo "NSID: "${NSID}
echo "CLUSTERID=${CLUSTERID}"

resp=$(
	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}?force=true
	); echo ${resp} | jq ''
    echo ""
