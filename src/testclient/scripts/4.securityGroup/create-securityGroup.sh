#!/bin/bash

function CallTB() {
	echo "- Create securityGroup in ${MCIRRegionNativeName}"

	resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/securityGroup -H 'Content-Type: application/json' -d @- <<EOF
        {
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
			"vNetId": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"description": "test description",
			"firewallRules": [
				{
					"FromPort": "1",
					"ToPort": "65535",
					"IPProtocol": "tcp",
					"Direction": "inbound",
					"CIDR": "0.0.0.0/0"
				},
				{
					"FromPort": "1",
					"ToPort": "65535",
					"IPProtocol": "udp",
					"Direction": "inbound",
					"CIDR": "0.0.0.0/0"
				},
				{
					"FromPort": "-1",
					"ToPort": "-1",
					"IPProtocol": "icmp",
					"Direction": "inbound",
					"CIDR": "0.0.0.0/0"
				}
			]
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
}

#function create_securityGroup() {

	echo "####################################################################"
	echo "## 4. SecurityGroup: Create"
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

				MCIRRegionNativeName=${RegionNativeName[$cspi,$cspj]}

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

#create_securityGroup
