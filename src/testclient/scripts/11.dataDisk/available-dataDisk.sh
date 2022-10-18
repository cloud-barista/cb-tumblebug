#!/bin/bash

echo "####################################################################"
echo "## 11. dataDisk: Get available dataDisks"
echo "####################################################################"

source ../init.sh

curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/vm/${CONN_CONFIG[$INDEX,$REGION]}-1/dataDisk?option=available | jq ''
