#!/bin/bash

echo "####################################################################"
echo "## 2. Namespace: Create (-x option for NameSpace Name)"
echo "####################################################################"

SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
cd $SCRIPT_DIR

source ../init.sh

if [ -z "$NSID" ]; then
	NSID=${OPTION01:-ns01}
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

