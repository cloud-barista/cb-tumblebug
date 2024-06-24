#!/bin/bash

echo "####################################################################"
echo "## 13. K8SCLUSTER: Set K8sNodeGroup's Autoscaling"
echo "####################################################################"

source ../init.sh

K8SCLUSTERID_ADD=${OPTION03:-1}

K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${INDEX}${REGION}${K8SCLUSTERID_ADD}
K8SNODEGROUPNAME="ng${INDEX}${REGION}${K8SCLUSTERID_ADD}"

echo "===================================================================="
echo "CSP=${CSP}"
echo "NSID=${NSID}"
echo "INDEX=${INDEX}"
echo "REGION=${REGION}"
echo "POSTFIX=${POSTFIX}"
echo "K8SNODEGROUPNAME=${K8SNODEGROUPNAME}"
echo "K8SCLUSTERID=${K8SCLUSTERID}"
echo "===================================================================="

req=$(cat << EOF
	{
		"onAutoScaling": "false"
	}
EOF
	); echo ${req} | jq ''


resp=$(
	curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/k8scluster/${K8SCLUSTERID}/k8snodegroup/${K8SNODEGROUPNAME}/onautoscaling -H 'Content-Type: application/json' -d @- <<EOF
		${req}
EOF
    ); echo ${resp} | jq ''
    echo ""

