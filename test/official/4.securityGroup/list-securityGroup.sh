#!/bin/bash

source ../conf.env
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

echo "####################################################################"
echo "## 4. SecurityGroup: List"
echo "####################################################################"


curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/securityGroup | json_pp #|| return 1

