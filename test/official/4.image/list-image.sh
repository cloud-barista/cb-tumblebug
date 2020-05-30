#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 4. image: List"
echo "####################################################################"


curl -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/image | json_pp #|| return 1
