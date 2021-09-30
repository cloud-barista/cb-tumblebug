#!/bin/bash

echo "####################################################################"
echo "## 7. spec: filter"
echo "####################################################################"

source ../init.sh

resp=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/filterSpecsByRange -H 'Content-Type: application/json' -d @- <<EOF
	    { 
		"connectionName": "aws",
		    "numvCPU": {
			    "min": 2,
			    "max": 2
		    }, 
		    "memGiB": {
			    "min": 4
		    },
		    "storageGiB": {
			    "max": 400
		    }
	    }
EOF
)
echo ${resp} | jq ''
echo ""
