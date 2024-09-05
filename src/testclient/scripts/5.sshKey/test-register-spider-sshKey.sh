#!/bin/bash

function CallTB() {
	echo "- Register sshKey in ${ResourceRegionNativeName}"

	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey?option=register -H 'Content-Type: application/json' -d \
		'{ 
			"connectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'", 
			"name": "'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'", 
			"cspResourceId": "",
			"fingerprint": "xx:c4:5a:ea:7f:c4:db:d5:80:80:92:47:7e:43:c9:2c:01:d3:ee:xx",
			"username": "cb-user",
			"publicKey": "",
			"privateKey": "-----BEGIN RSA PRIVATE KEY-----\nMIIE....Kplg==\n-----END RSA PRIVATE KEY-----"
		}' | jq ''
}

#function register_sshKey() {

	echo "####################################################################"
	echo "## 5. sshKey: Register"
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

#register_sshKey
