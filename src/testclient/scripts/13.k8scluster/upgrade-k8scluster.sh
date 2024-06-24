#!/bin/bash

echo "####################################################################"
echo "## 13. K8SCLUSTER: Upgrade"
echo "####################################################################"

source ../init.sh

if [ -n "${K8S_UPGRADE_VERSION[$INDEX,$REGION]}" ]; then
	VERSION=${K8S_UPGRADE_VERSION[$INDEX,$REGION]}
else
	echo "You need to specify K8S_UPGRADE_VERION[\$IX,\$IY] in conf.env!!!"
	exit
fi

K8SCLUSTERID_ADD=${OPTION03:-1}

K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${INDEX}${REGION}${K8SCLUSTERID_ADD}

echo "===================================================================="
echo "CSP=${CSP}"
echo "NSID=${NSID}"
echo "INDEX=${INDEX}"
echo "REGION=${REGION}"
echo "POSTFIX=${POSTFIX}"
echo "VERSION=${VERSION}"
echo "K8SCLUSTERID=${K8SCLUSTERID}"
echo "===================================================================="


req=$(cat << EOF
	{
		"version": "${VERSION}"
	}
EOF
	); echo ${req} | jq ''

resp=$(
	curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/k8scluster/${K8SCLUSTERID}/upgrade -H 'Content-Type: application/json' -d @- <<EOF
		${req}
EOF
    ); echo ${resp} | jq ''
    echo ""

