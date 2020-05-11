#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 6. VM: List MCIS"
echo "####################################################################"


curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/mcis | json_pp
