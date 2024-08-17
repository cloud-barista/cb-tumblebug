#!/bin/bash

echo "####################################################################"
echo "## 0. Namespace: Get (need input parameter: [-x namespace])"
echo "####################################################################"

source ../init.sh

NSID=${OPTION01:-default}

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID | jq ''
echo ""
