#!/bin/bash

echo "####################################################################"
echo "## 7. spec: filter"
echo "####################################################################"

source ../init.sh

resp=$(
    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/testSortSpecs -H 'Content-Type: application/json' -d @- <<EOF
	    { 
		    "numvCPU": 4
	    }
EOF
)
echo ${resp} | jq ''
echo ""
