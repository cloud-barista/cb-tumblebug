#!/bin/bash

#function create_ns() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	TestSetFile=${4:-../testSet.env}
    
    FILE=$TestSetFile
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## 2. Config: Create or Update (Param1: Key, Param2: Value)"
	echo "####################################################################"

	KEY=${1}
	VALUE=${2}

	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/config -H 'Content-Type: application/json' -d \
		'{
			"name": "'${KEY}'",
			"value": "'${VALUE}'"
		}' | json_pp #|| return 1
#}

#create_ns