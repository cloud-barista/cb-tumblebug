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

NUMVM=${OPTION01:-1}
CLUSTERID_ADD=${OPTION03:-1}

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}${CLUSTERID_ADD}

NODEGROUPNAME="ng${INDEX}${REGION}${CLUSTERID_ADD}"
if [ -n "${CONTAINER_IMAGE_NAME[$INDEX,$REGION]}" ]; then
	NODEIMAGEID="k8s-${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}"
else
	NODEIMAGEID="${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}"
fi

DesiredNodeSize=$NUMVM
MinNodeSize="1"
MaxNodeSize=$NUMVM

echo "===================================================================="
echo "CSP=${CSP}"
echo "NSID=${NSID}"
echo "INDEX=${INDEX}"
echo "REGION=${REGION}"
echo "POSTFIX=${POSTFIX}"
echo "NAME=${NODEGROUPNAME}"
echo "IMAGEID=${NODEIMAGEID}"
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
