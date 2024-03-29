#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Set Autoscaling"
echo "####################################################################"

source ../init.sh

CLUSTERID_ADD=${OPTION03:-1}

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}${CLUSTERID_ADD}
NODEGROUPNAME="ng${INDEX}${REGION}${CLUSTERID_ADD}"

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

