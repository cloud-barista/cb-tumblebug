#!/bin/bash

echo "####################################################################"
echo "## Gen csv file from script config"
echo "####################################################################"

source ../init.sh

PRINT="ProviderName,CONN_CONFIG,RegionNativeName,NativeRegionNativeName,RegionLocation,DriverLibFileName,DriverName"
echo "${PRINT}"
echo "${PRINT}" >./cloudconnection.csv

INDEXX=${TotalNumCSP}
for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${TotalNumRegion[$cspi]}
	for ((cspj = 1; cspj <= INDEXY; cspj++)); do
		PRINT="${ProviderName[$cspi]},${CONN_CONFIG[$cspi,$cspj]},${RegionNativeName[$cspi,$cspj]},${RegionVal01[$cspi,$cspj]},${RegionLocation[$cspi,$cspj]},${DriverLibFileName[$cspi]},${DriverName[$cspi]}"
		echo "$PRINT"
		echo "$PRINT" >>./cloudconnection.csv
	done
done
