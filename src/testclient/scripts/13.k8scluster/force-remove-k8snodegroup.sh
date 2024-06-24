#!/bin/bash

echo "####################################################################"
echo "## 13. K8SCLUSTER: Remove K8sNodeGroup"
echo "####################################################################"

source ../init.sh

K8SCLUSTERID_ADD=${OPTION03:-1}

K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${INDEX}${REGION}${K8SCLUSTERID_ADD}
K8SNODEGROUPNAME="ng${INDEX}${REGION}${K8SCLUSTERID_ADD}"

echo "NSID: "${NSID}
echo "K8SCLUSTERID=${K8SCLUSTERID}"
echo "K8SNODEGROUPNAME=${K8SNODEGROUPNAME}"

resp=$(
	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/k8scluster/${K8SCLUSTERID}/k8snodegroup/${K8SNODEGROUPNAME}?force=true
	); echo ${resp} | jq ''
    echo ""
