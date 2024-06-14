#!/bin/bash

echo "####################################################################"
echo "## 13. K8SCLUSTER: List"
echo "####################################################################"

source ../init.sh

echo "NSID: "${NSID}

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/k8scluster | jq ''
