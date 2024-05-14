#!/bin/bash

function CallTB() {
	echo "- Register image in ${MCIRRegionNativeName}"

	resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/image?action=registerWithId -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}", 
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"cspImageId": "${IMAGE_NAME[$INDEX,$REGION]}",
			"description": "Canonical, Ubuntu, 18.04 LTS, amd64 bionic",
			"guestOS": "Ubuntu"
		}
EOF
	); echo ${resp} | jq ''
	echo ""

	if [ -n "${CONTAINER_IMAGE_NAME[$INDEX,$REGION]}" ]; then
	    echo "- Register K8s node image in ${MCIRRegionNativeName}"

	    resp=$(
	    curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/image?action=registerWithId -H 'Content-Type: application/json' -d @- <<EOF
		    { 
			    "connectionName": "${CONN_CONFIG[$INDEX,$REGION]}", 
			    "name": "k8s-${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			    "cspImageId": "${CONTAINER_IMAGE_NAME[$INDEX,$REGION]}",
			    "description": "${CONTAINER_IMAGE_TYPE[$INDEX,$REGION]}",
			    "guestOS": "Ubuntu"
		    }
EOF
	    ); echo ${resp} | jq ''
	fi

}

#function registerImageWithId() {

	echo "####################################################################"
	echo "## 6. image: Register"
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

				MCIRRegionNativeName=${CONN_CONFIG[$cspi,$cspj]}

				INDEX=$cspi
				REGION=$cspj
				CallTB
			done
		done
		wait

	else
		echo ""
		
		MCIRRegionNativeName=${CONN_CONFIG[$INDEX,$REGION]}

		CallTB

	fi
	
#}

#registerImageWithId
