#!/bin/bash

echo "####################################################################"
echo "## Gen csv file from script config"
echo "####################################################################"

source ../init.sh

PRINT="ProviderName,CONN_CONFIG,RegionName,RegionLocation,DriverLibFileName,DriverName"
echo "${PRINT}"
echo "${PRINT}" >./cloudconnection.csv

INDEXX=${NumCSP}
for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${NumRegion[$cspi]}
	for ((cspj = 1; cspj <= INDEXY; cspj++)); do
		PRINT="${ProviderName[$cspi]},${CONN_CONFIG[$cspi,$cspj]},${RegionName[$cspi,$cspj]},${RegionLocation[$cspi,$cspj]},${DriverLibFileName[$cspi]},${DriverName[$cspi]}"
		echo "$PRINT"
		echo "$PRINT" >>./cloudconnection.csv
	done
done
