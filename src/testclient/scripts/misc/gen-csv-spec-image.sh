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

PRINT="providerName,regionName,connectionName,cspSpecName,CostPerHour,evaluationScore01,evaluationScore02,evaluationScore03,evaluationScore04,evaluationScore05,evaluationScore06,evaluationScore07,evaluationScore08,evaluationScore09,evaluationScore10,rootDiskType,rootDiskSize"
echo "${PRINT}"
echo "${PRINT}" >./cloudspec.csv

INDEXX=${TotalNumCSP}
for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${TotalNumRegion[$cspi]}
	for ((cspj = 1; cspj <= INDEXY; cspj++)); do
		PRINT="${ProviderName[$cspi]},${RegionName[$cspi,$cspj]},${CONN_CONFIG[$cspi,$cspj]},${SPEC_NAME[$cspi,$cspj]},,,,,,,,,,,${DISK_TYPE[$cspi,$cspj]},${DISK_SIZE[$cspi,$cspj]}"
		echo "$PRINT"
		echo "$PRINT" >>./cloudspec.csv
	done
done