#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Add NodeGroup"
echo "####################################################################"

source ../init.sh

if [[ -z "${DISK_TYPE[$INDEX,$REGION]}" ]]; then
        RootDiskType="default"
else
        RootDiskType="${DISK_TYPE[$INDEX,$REGION]}"
fi

if [[ -z "${DISK_SIZE[$INDEX,$REGION]}" ]]; then
        RootDiskSize="default"
else
        RootDiskSize="${DISK_SIZE[$INDEX,$REGION]}"
fi

if [ "$CSP" == "azure" ]; then
	NODEGROUPNAME="new${INDEX}${REGION}"
	NODEIMAGEID=""
else
	#NODEGROUPNAME="${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}"
	NODEGROUPNAME="new${INDEX}${REGION}"
	NODEIMAGEID="${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}"
fi 

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}

NUMVM=${OPTION01:-1}

DesiredNodeSize=$NUMVM
MinNodeSize="1"
MaxNodeSize=$NUMVM

echo "===================================================================="
echo "CSP=${CSP}"
echo "NSID=${NSID}"
echo "INDEX=${INDEX}"
echo "REGION=${REGION}"
echo "POSTFIX=${POSTFIX}"
echo "RootDiskType=${RootDiskType}"
echo "RootDiskSize=${RootDiskSize}"
echo "DesiredNodeSize=${DesiredNodeSize}"
echo "MinNodeSize=${MinNodeSize}"
echo "MaxNodeSize=${MaxNodeSize}"
echo "CLUSTERID=${CLUSTERID}"
echo "===================================================================="

resp=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}/nodegroup -H 'Content-Type: application/json' -d @- <<EOF
	{
		"name": "${NODEGROUPNAME}",
		"imageId": "${NODEIMAGEID}",
		"specId": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
		"rootDiskType": "${RootDiskType}",
		"rootDiskSize": "${RootDiskSize}",
		"sshKeyId": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",

		"onAutoScaling": "true",
		"desiredNodeSize": "${DesiredNodeSize}",
		"minNodeSize": "${MinNodeSize}",
		"maxNodeSize": "${MaxNodeSize}"
	}
EOF
    ); echo ${resp} | jq ''
    echo ""
