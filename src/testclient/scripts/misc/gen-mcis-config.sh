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


VAR1=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$nsForSystem/resources/spec -H 'Content-Type: application/json' )

for row in $(echo "${VAR1}" | jq -r '.spec[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

  id=$(_jq '.id')
  rootDiskType=$(_jq '.rootDiskType')
  rootDiskSize=$(_jq '.rootDiskSize')
  echo "  {" >>./mcisconfig.json
  echo "    \"commonImage\": \"ubuntu18.04\"," >>./mcisconfig.json
	echo "    \"commonSpec\": \"$id\","  >>./mcisconfig.json
  echo "    \"rootDiskType\": \"$rootDiskType\","  >>./mcisconfig.json
  echo "    \"rootDiskSize\": \"$rootDiskSize\""  >>./mcisconfig.json
  echo "  },"  >>./mcisconfig.json
done

sed -i '$ d' ./mcisconfig.json
echo "  }"  >>./mcisconfig.json

echo "]}" >>./mcisconfig.json

cat ./mcisconfig.json