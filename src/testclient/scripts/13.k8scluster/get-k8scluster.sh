#!/bin/bash

echo "####################################################################"
echo "## 13. K8SCLUSTER: Get"
echo "####################################################################"

source ../init.sh

K8SCLUSTERID_ADD=${OPTION03:-1}

K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${INDEX}${REGION}${K8SCLUSTERID_ADD}

echo "NSID: "${NSID}
echo "K8SCLUSTERID=${K8SCLUSTERID}"

resp=$(
	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/k8scluster/${K8SCLUSTERID}
	); echo ${resp} | jq '.'
    echo ""
