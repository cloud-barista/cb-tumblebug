#!/bin/bash

echo "####################################################################"
echo "## Load common Image and Spec from asset files (takes 10~120 seconds)"
echo "## (assets/cloudspec.csv, assets/cloudimage.csv)"
echo "####################################################################"

SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
cd $SCRIPT_DIR

source ../init.sh

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/loadCommonResource | jq ''
echo ""
