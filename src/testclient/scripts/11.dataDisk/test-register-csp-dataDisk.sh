#!/bin/bash

function CallTB() {
	echo "- Register dataDisk in ${ResourceRegionNativeName}"

	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/dataDisk?option=register -H 'Content-Type: application/json' -d \
		'{ 
			"connectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'", 
			"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'", 
			"cspDataDiskId": "vol-xxxx"
		}' | jq ''
}

#function register_dataDisk() {

	echo "####################################################################"
	echo "## 11. dataDisk: Register"
	echo "####################################################################"

	source ../init.sh

	if [ "${INDEX}" == "0" ]; then
		echo "[Parallel execution for all CSP regions]"
		INDEXX=${NumCSP}
		for ((cspi = 1; cspi <= INDEXX; cspi++)); do
			INDEXY=${NumRegion[$cspi]}
			CSP=${CSPType[$cspi]}
			echo "[$cspi] $CSP details"
			for ((cspj = 1; cspj <= INDEXY; cspj++)); do
				echo "[$cspi,$cspj] ${RegionNativeName[$cspi,$cspj]}"

				ResourceRegionNativeName=${RegionNativeName[$cspi,$cspj]}

				INDEX=$cspi
				REGION=$cspj
				CallTB
			done
		done
		wait

	else
		echo ""
		
		ResourceRegionNativeName=${CONN_CONFIG[$INDEX,$REGION]}

		CallTB

	fi
	
#}

#register_dataDisk
