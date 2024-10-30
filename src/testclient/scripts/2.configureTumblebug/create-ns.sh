#!/bin/bash

echo "####################################################################"
echo "## 2. Namespace: Create (-x option for NameSpace Name)"
echo "####################################################################"

SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`

echo $SCRIPT_DIR
cd $SCRIPT_DIR

source $TB_ROOT_PATH/src/testclient/scripts/init.sh

if [ ! -z "$OPTION01" ]; then
	NSID=$OPTION01
fi

if [ -z "$NSID" ]; then
	NSID="default"
fi

req=$(cat <<EOF
	{
		"name": "$NSID",
		"description": "NameSpace for General Testing"
	}
EOF
	); echo ${req} | jq '.'

resp=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns -H 'Content-Type: application/json' -d @- <<EOF
		${req}
EOF
	); echo ${resp} | jq '.'
	echo ""

