#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Upgrade"
echo "####################################################################"

source ../init.sh

<<COMMENT
if [ "$CSP" == "azure" ]; then
	VERSION=${OPTION02:-1.25.11}
elif [ "$CSP" == "tencent" ]; then
	#VERSION=${OPTION02:-1.20.6}
	VERSION=${OPTION02:-1.22.5}
	#VERSION=${OPTION02:-1.24.4}
	#VERSION=${OPTION02:-1.26.1}
elif [ "$CSP" == "alibaba" ]; then
	#VERSION=${OPTION02:-1.24.6-aliyun.1}
	#VERSION=${OPTION02:-1.26.3-aliyun.1}
	VERSION=${OPTION02:-1.28.3-aliyun.1}
elif [ "$CSP" == "nhncloud" ]; then
	#VERSION=${OPTION02:-1.24.3}
	#VERSION=${OPTION02:-1.25.4}
	#VERSION=${OPTION02:-1.26.3}
	VERSION=${OPTION02:-v1.27.3}
else
	VERSION=${OPTION02:-1.25.11}
fi 
COMMENT

if [ -n "${K8S_UPGRADE_VERSION[$INDEX,$REGION]}" ]; then
	VERSION=${K8S_UPGRADE_VERSION[$INDEX,$REGION]}
else
	echo "You need to specify K8S_UPGRADE_VERION[\$IX,\$IY] in conf.env!!!"
	exit
fi

CLUSTERID_ADD=${OPTION03:-1}

CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}${CLUSTERID_ADD}

echo "NSID: "${NSID}
echo "CLUSTERID=${CLUSTERID}"

resp=$(
	curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}/upgrade -H 'Content-Type: application/json' -d @- <<EOF
	{
		"version": "${VERSION}"
	}
EOF
    ); echo ${resp} | jq ''
    echo ""

