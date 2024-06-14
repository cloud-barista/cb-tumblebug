#!/bin/bash

echo "####################################################################"
echo "## 13. K8SCLUSTER: Add K8sNodeGroup"
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
K8SCLUSTERID_ADD=${OPTION03:-1}

K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${INDEX}${REGION}${K8SCLUSTERID_ADD}

K8SNODEGROUPNAME="ng${INDEX}${REGION}${K8SCLUSTERID_ADD}"
if [ -n "${CONTAINER_IMAGE_NAME[$INDEX,$REGION]}" ]; then
	#NODEIMAGEID="k8s-${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}"
	NODEIMAGEID="default"
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
echo "NAME=${K8SNODEGROUPNAME}"
echo "IMAGEID=${NODEIMAGEID}"
echo "RootDiskType=${RootDiskType}"
echo "RootDiskSize=${RootDiskSize}"
echo "DesiredNodeSize=${DesiredNodeSize}"
echo "MinNodeSize=${MinNodeSize}"
echo "MaxNodeSize=${MaxNodeSize}"
echo "K8SCLUSTERID=${K8SCLUSTERID}"
echo "===================================================================="

resp=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/k8scluster/${K8SCLUSTERID}/k8snodegroup -H 'Content-Type: application/json' -d @- <<EOF
	{
		"name": "${K8SNODEGROUPNAME}",
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
