#!/bin/bash

echo "####################################################################"
echo "## 13. K8SCLUSTER: Check NodeGroups On K8s Creation"
echo "####################################################################"

source ../init.sh

K8SCLUSTERID_ADD=${OPTION03:-1}

K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${INDEX}${REGION}${K8SCLUSTERID_ADD}

echo "NSID: "${NSID}
echo "K8SCLUSTERID=${K8SCLUSTERID}"

PROVIDERNAME="alibaba"
PROVIDERNAME="nhncloud"

resp=$(
	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/checkNodeGroupsOnK8sCreation?providerName=${PROVIDERNAME}
	); echo ${resp} | jq '.'
    echo ""
