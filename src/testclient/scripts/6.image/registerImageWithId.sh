#!/bin/bash

function CallTB() {
	echo "- Register image in ${MCIRRegionName}"

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
}

#function registerImageWithId() {

	echo "####################################################################"
	echo "## 6. image: Register"
	echo "####################################################################"

	source ../init.sh

	if [ "${INDEX}" == "0" ]; then
		echo "[Parallel excution for all CSP regions]"

		INDEXX=${NumCSP}
		for ((cspi = 1; cspi <= INDEXX; cspi++)); do
			echo $i
			INDEXY=${NumRegion[$cspi]}
			CSP=${CSPType[$cspi]}
			for ((cspj = 1; cspj <= INDEXY; cspj++)); do
				# INDEX=$(($INDEX+1))

				echo $j
				INDEX=$cspi
				REGION=$cspj
				echo $CSP
				echo $REGION
				echo ${RegionName[$cspi,$cspj]}
				MCIRRegionName=${RegionName[$cspi,$cspj]}

				CallTB

			done

		done
		wait

	else
		echo ""
		
		MCIRRegionName=${CONN_CONFIG[$INDEX,$REGION]}

		CallTB

	fi
	
#}

#registerImageWithId
