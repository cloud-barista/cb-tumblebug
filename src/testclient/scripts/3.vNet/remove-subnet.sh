#!/bin/bash

function CallTB() {
	echo "- Remove subnet in ${ResourceRegionNativeName}"

	CIDRNum=$(($INDEX+1))
	CIDRDiff=$(($CIDRNum*$REGION))
	CIDRDiff=$(($CIDRDiff%254))
	# CIDRDiff=$(($CIDRDiff+1))

    resp=$(
        curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/resources/vNet/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}/subnet/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}-${CIDRDiff} -H 'Content-Type: application/json' 
    ); echo ${resp} | jq '.'
    echo ""
}

#function create_subnet() {

	echo "####################################################################"
	echo "## 3. subnet: Remove"
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

#create_subnet
