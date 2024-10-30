#!/bin/bash

echo "####################################################################"
echo "## 13. K8SCLUSTER: Get from CB-Spider"
echo "####################################################################"

source ../init.sh

K8SCLUSTERID_ADD=${OPTION03:-1}

K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${INDEX}${REGION}${K8SCLUSTERID_ADD}

echo "NSID: "${NSID}
echo "K8SCLUSTERID=${K8SCLUSTERID}"

resp=$(
	curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/cluster/${NSID}-${K8SCLUSTERID} -H 'Content-Type: application/json' -d @- <<EOF
        { 
		"ConnectionName": "${CONN_CONFIG[$INDEX,$REGION]}"
	}
EOF
	); echo ${resp} | jq '.'
    echo ""
