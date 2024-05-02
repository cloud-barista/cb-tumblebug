#!/bin/bash

function CallTB() {
	echo "- Delete firewallRules in ${MCIRRegionNativeName}"

	resp=$(
        curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/resources/securityGroup/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}/rules -H 'Content-Type: application/json' -d @- <<EOF
        {
			"firewallRules": [
				{
					"FromPort": "1",
					"ToPort": "65534",
					"IPProtocol": "tcp",
					"Direction": "inbound",
					"CIDR": "0.0.0.0/0"
				},
				{
					"FromPort": "1",
					"ToPort": "65534",
					"IPProtocol": "udp",
					"Direction": "inbound",
					"CIDR": "0.0.0.0/0"
				}
			]
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
}

#function create_firewallRules() {

	echo "####################################################################"
	echo "## 4. firewallRules: Delete"
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

#create_firewallRules
