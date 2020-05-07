#!/bin/bash

source ../conf.env

INDEX=${1}

echo "####################################################################"
echo "## 4. VM: Terminate(Delete)"
echo "####################################################################"

curl -sX DELETE http://localhost:1323/tumblebug/ns/$NS_ID/mcis/MCIS-0$INDEX | json_pp || return 1

