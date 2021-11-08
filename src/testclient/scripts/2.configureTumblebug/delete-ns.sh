#!/bin/bash

echo "####################################################################"
echo "## 0. Namespace: Delete (need input parameter: [-x namespace])"
echo "####################################################################"

source ../init.sh

NSID=${OPTION01:-ns01}

curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID | jq ''
echo ""
