#!/bin/bash

echo "####################################################################"
echo "## Gen MCIS config using all specs in the system ns"
echo "####################################################################"

source ../init.sh

nsForSystem="system-purpose-common-ns"

PRINT="{
  \"description\": \"Made in CB-TB\",
  \"installMonAgent\": \"no\",
  \"label\": \"DynamicVM\",
  \"name\": \"mcis01\",
  \"systemLabel\": \"\",
  \"vm\": ["

echo "${PRINT}"
echo "${PRINT}" >./mcisconfig.json


VAR1=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$nsForSystem/resources/spec?option=id -H 'Content-Type: application/json' )

VMARRAY=$(jq -r '.output[]' <<<"$VAR1")
#echo "${VMARRAY}"

for d in ${VMARRAY}
do
  echo "  {" >>./mcisconfig.json
  echo "    \"commonImage\": \"ubuntu18.04\"," >>./mcisconfig.json
	echo "    \"commonSpec\": \"$d\""  >>./mcisconfig.json
  echo "  },"  >>./mcisconfig.json
done
sed -i '$ d' ./mcisconfig.json
echo "  }"  >>./mcisconfig.json

echo "]}" >>./mcisconfig.json

cat ./mcisconfig.json