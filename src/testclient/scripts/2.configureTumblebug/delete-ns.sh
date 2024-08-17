#!/bin/bash

echo "####################################################################"
echo "## 0. Namespace: Delete (need input parameter: [-x namespace])"
echo "####################################################################"

source ../init.sh

NSID=${OPTION01:-default}

curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID | jq ''
echo ""
