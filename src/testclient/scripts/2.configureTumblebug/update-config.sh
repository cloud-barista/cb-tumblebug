#!/bin/bash

#function create_ns() {

	source ../init.sh

	echo "####################################################################"
	echo "## 2. Config: Create or Update (Param1: Key, Param2: Value)"
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
    ); echo ${resp} | jq ''
    echo ""
#}

#create_ns