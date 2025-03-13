#!/bin/bash

echo "####################################################################"
echo "## 7. spec: filter"
echo "####################################################################"

source ../init.sh

resp=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/filterSpecsByRange -H 'Content-Type: application/json' -d @- <<EOF
	    { 
		"connectionName": "aws",
		    "vCPU": {
			    "min": 2,
			    "max": 2
		    }, 
		    "memoryGiB": {
			    "min": 4
		    },
		    "diskSizeGB": {
			    "max": 400
		    }
	    }
EOF
)
echo ${resp} | jq '.'
echo ""
