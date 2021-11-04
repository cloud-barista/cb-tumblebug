#!/bin/bash

echo "####################################################################"
echo "## Gen csv file from script config"
echo "####################################################################"

source ../init.sh

PRINT="ProviderName,connectionName,cspImageId,OsType"
echo "${PRINT}"
echo "${PRINT}" >./cloudimage.csv

INDEXX=${TotalNumCSP}
for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${TotalNumRegion[$cspi]}
	for ((cspj = 1; cspj <= INDEXY; cspj++)); do
		PRINT="${ProviderName[$cspi]},${CONN_CONFIG[$cspi,$cspj]},${IMAGE_NAME[$cspi,$cspj]},${IMAGE_TYPE[$cspi,$cspj]}"
		echo "$PRINT"
		echo "$PRINT" >>./cloudimage.csv
	done
done
