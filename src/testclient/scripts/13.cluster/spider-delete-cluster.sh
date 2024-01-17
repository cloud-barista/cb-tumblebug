#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Delete from CB-Spider"
echo "####################################################################"

source ../init.sh

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}

echo "NSID: "${NSID}
echo "CLUSTERID=${CLUSTERID}"

resp=$(
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/cluster/${NSID}-${CLUSTERID} -H 'Content-Type: application/json' -d @- <<EOF
        { 
		"ConnectionName": "${CONN_CONFIG[$INDEX,$REGION]}"
	}
EOF
	); echo ${resp} | jq ''
    echo ""
