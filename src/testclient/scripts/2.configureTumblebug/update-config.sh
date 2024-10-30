#!/bin/bash

source ../init.sh

echo "####################################################################"
echo "## Config: Create or Update (-x: Key, -y: Value)"
echo "####################################################################"

KEY=${OPTION01}
VALUE=${OPTION02}

resp=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/config -H 'Content-Type: application/json' -d @- <<EOF
		{
			"name": "${KEY}",
			"value": "${VALUE}"
		}
EOF
)
echo ${resp} | jq '.'
echo ""
