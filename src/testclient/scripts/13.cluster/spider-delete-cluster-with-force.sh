#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Delete from CB-Spider"
echo "####################################################################"

source ../init.sh

CLUSTERID_ADD=${OPTION03:-1}

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}${CLUSTERID_ADD}

echo "NSID: "${NSID}
echo "CLUSTERID=${CLUSTERID}"

resp=$(
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/cluster/${NSID}-${CLUSTERID}?force=true -H 'Content-Type: application/json' -d @- <<EOF
        { 
		"ConnectionName": "${CONN_CONFIG[$INDEX,$REGION]}"
	}
EOF
	); echo ${resp} | jq ''
    echo ""
