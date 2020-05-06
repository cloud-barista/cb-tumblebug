#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 4. VM: Terminate(Delete)"
echo "####################################################################"
curl -sX DELETE http://localhost:1323/tumblebug/ns/$NS_ID/mcis/MCIS-01 | json_pp || return 1

