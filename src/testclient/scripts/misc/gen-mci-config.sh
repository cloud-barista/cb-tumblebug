#!/bin/bash

echo "####################################################################"
echo "## Gen Infra config using all specs in the system ns"
echo "####################################################################"

source ../init.sh

nsForSystem="system"

PRINT="{
  \"description\": \"Made in CB-TB\",
  \"installMonAgent\": \"no\",
  \"label\": \"DynamicVM\",
  \"name\": \"infra01\",
  \"systemLabel\": \"\",
  \"vm\": ["

echo "${PRINT}"
echo "${PRINT}" >./infraconfig.json


VAR1=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$nsForSystem/resources/spec -H 'Content-Type: application/json' )

for row in $(echo "${VAR1}" | jq -r '.spec[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

  id=$(_jq '.id')
  rootDiskType=$(_jq '.rootDiskType')
  rootDiskSize=$(_jq '.rootDiskSize')
  echo "  {" >>./infraconfig.json
  echo "    \"imageId\": \"ubuntu18.04\"," >>./infraconfig.json
	echo "    \"specId\": \"$id\","  >>./infraconfig.json
  echo "    \"rootDiskType\": \"$rootDiskType\","  >>./infraconfig.json
  echo "    \"rootDiskSize\": \"$rootDiskSize\""  >>./infraconfig.json
  echo "  },"  >>./infraconfig.json
done

sed -i '$ d' ./infraconfig.json
echo "  }"  >>./infraconfig.json

echo "]}" >>./infraconfig.json

cat ./infraconfig.json