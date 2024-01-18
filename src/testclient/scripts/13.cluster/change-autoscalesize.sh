#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Change Autoscale Size"
echo "####################################################################"

source ../init.sh

if [ "$CSP" == "azure" ]; then
	NODEGROUPNAME="new${INDEX}${REGION}"
else
	NODEGROUPNAME="new${INDEX}${REGION}"
fi 

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}

NUMVM=${OPTION01:-1}

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
echo "CLUSTERID=${CLUSTERID}"
echo "===================================================================="

resp=$(
	curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}/nodegroup/${NODEGROUPNAME}/autoscalesize -H 'Content-Type: application/json' -d @- <<EOF
	{
		"desiredNodeSize": "${DesiredNodeSize}",
		"minNodeSize": "${MinNodeSize}",
		"maxNodeSize": "${MaxNodeSize}"
	}
EOF
    ); echo ${resp} | jq ''
    echo ""

