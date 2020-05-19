#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 5. spec: List"
echo "####################################################################"


curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/spec | json_pp #|| return 1
