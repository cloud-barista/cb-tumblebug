#!/bin/bash

function CallTB() {
	echo "- Create vNet in ${MCIRRegionName}"

	CIDRNum=$(($INDEX+1))
	CIDRDiff=$(($CIDRNum*$REGION))
	CIDRDiff=$(($CIDRDiff%254))

	CidrBlock="10.${CIDRDiff}.0.0/16"
	
	# Original
	# IPv4CIDR01="10.${CIDRDiff}.0.0/18"
	# IPv4CIDR02="10.${CIDRDiff}.64.0/18"

	# Modified by OH (For extra range for NLB subnet )
	IPv4CIDR01="10.${CIDRDiff}.0.0/28"
	IPv4CIDR02="10.${CIDRDiff}.64.0/28"

	# CidrBlock="10.1.0.0/16" # for a temporal test for a limited CSP.
	# IPv4CIDR01="10.1.0.0/18"
	# IPv4CIDR02="10.1.64.0/18"

    resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/vNet -H 'Content-Type: application/json' -d @- <<EOF
        {
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
			"cidrBlock": "${CidrBlock}",
			"subnetInfoList": [ {
				"Name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
				"IPv4_CIDR": "${IPv4CIDR01}"
			}, {
				"Name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}-01",
				"IPv4_CIDR": "${IPv4CIDR02}"
			} ]
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
}

#function create_vNet() {

	echo "####################################################################"
	echo "## 3. vNet: Create"
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
                echo "[$cspi,$cspj] ${RegionName[$cspi,$cspj]}"
				
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

#create_vNet