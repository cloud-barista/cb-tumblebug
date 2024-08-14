#!/bin/bash

echo "####################################################################"
echo "## Gen MCI config using all specs in the system ns"
echo "####################################################################"

source ../init.sh

nsForSystem="system-purpose-common-ns"

PRINT="{
  \"description\": \"Made in CB-TB\",
  \"installMonAgent\": \"no\",
  \"label\": \"DynamicVM\",
  \"name\": \"mci01\",
  \"systemLabel\": \"\",
  \"vm\": ["

echo "${PRINT}"
echo "${PRINT}" >./mciconfig.json


VAR1=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$nsForSystem/resources/spec -H 'Content-Type: application/json' )

for row in $(echo "${VAR1}" | jq -r '.spec[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

  id=$(_jq '.id')
  rootDiskType=$(_jq '.rootDiskType')
  rootDiskSize=$(_jq '.rootDiskSize')
  echo "  {" >>./mciconfig.json
  echo "    \"commonImage\": \"ubuntu18.04\"," >>./mciconfig.json
	echo "    \"commonSpec\": \"$id\","  >>./mciconfig.json
  echo "    \"rootDiskType\": \"$rootDiskType\","  >>./mciconfig.json
  echo "    \"rootDiskSize\": \"$rootDiskSize\""  >>./mciconfig.json
  echo "  },"  >>./mciconfig.json
done

sed -i '$ d' ./mciconfig.json
echo "  }"  >>./mciconfig.json

echo "]}" >>./mciconfig.json

cat ./mciconfig.json