#!/bin/bash

echo "####################################################################"
echo "## 13. K8SCLUSTER: Change Autoscale Size"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}
K8SCLUSTERID_ADD=${OPTION03:-1}

K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${INDEX}${REGION}${K8SCLUSTERID_ADD}
K8SNODEGROUPNAME="ng${INDEX}${REGION}${K8SCLUSTERID_ADD}"

DesiredNodeSize=$(($NUMVM+1))
MinNodeSize=$(($NUMVM+1))
MaxNodeSize=$(($NUMVM+2))

echo "===================================================================="
echo "CSP=${CSP}"
echo "NSID=${NSID}"
echo "INDEX=${INDEX}"
echo "REGION=${REGION}"
echo "POSTFIX=${POSTFIX}"
echo "DesiredNodeSize=${DesiredNodeSize}"
echo "MinNodeSize=${MinNodeSize}"
echo "MaxNodeSize=${MaxNodeSize}"
echo "K8SCLUSTERID=${K8SCLUSTERID}"
echo "===================================================================="

resp=$(
	curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/k8scluster/${K8SCLUSTERID}/k8snodegroup/${K8SNODEGROUPNAME}/autoscalesize -H 'Content-Type: application/json' -d @- <<EOF
	{
		"desiredNodeSize": "${DesiredNodeSize}",
		"minNodeSize": "${MinNodeSize}",
		"maxNodeSize": "${MaxNodeSize}"
	}
EOF
    ); echo ${resp} | jq ''
    echo ""

