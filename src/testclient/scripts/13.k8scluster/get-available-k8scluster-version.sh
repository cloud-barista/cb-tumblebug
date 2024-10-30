#!/bin/bash

echo "####################################################################"
echo "## 13. K8SCLUSTER: Get Available Version"
echo "####################################################################"

source ../init.sh

K8SCLUSTERID_ADD=${OPTION03:-1}

K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${INDEX}${REGION}${K8SCLUSTERID_ADD}

echo "NSID: "${NSID}
echo "K8SCLUSTERID=${K8SCLUSTERID}"

PROVIDERNAME="alibaba"
#REGIONNAME="me-east-1"
REGIONNAME="ap-south-1"

resp=$(
	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/availableK8sClusterVersion?providerName=${PROVIDERNAME}\&regionName=${REGIONNAME}
	); echo ${resp} | jq '.'
    echo ""
