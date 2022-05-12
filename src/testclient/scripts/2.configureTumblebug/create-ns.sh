#!/bin/bash

echo "####################################################################"
echo "## 2. Namespace: Create (-x option for NameSpace Name)"
echo "####################################################################"

SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
cd $SCRIPT_DIR

source $CBTUMBLEBUG_ROOT/src/testclient/scripts/init.sh

if [ ! -z "$OPTION01" ]; then
	NSID=$OPTION01
fi

if [ -z "$NSID" ]; then
	NSID="ns01"
fi

resp=$(
    curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns -H 'Content-Type: application/json' -d @- <<EOF
        {
			"name": "$NSID",
			"description": "NameSpace for General Testing"
		}
EOF
)
echo ${resp} | jq ''
echo ""

