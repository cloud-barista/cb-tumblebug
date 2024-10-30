#!/bin/bash

function CallTB() {
	echo "- Create vNet in ${ResourceRegionNativeName}"

	CIDRNum=$(($INDEX+1))
	CIDRDiff=$(($CIDRNum*$REGION))
	CIDRDiff=$(($CIDRDiff%254))

	CidrBlock="10.${CIDRDiff}.0.0/16"
	IPv4CIDR01="10.${CIDRDiff}.0.0/18"
	IPv4CIDR02="10.${CIDRDiff}.64.0/18"

	# CidrBlock="10.1.0.0/16" # for a temporal test for a limited CSP.
	# IPv4CIDR01="10.1.0.0/18"
	# IPv4CIDR02="10.1.64.0/18"

	ZONE1=""
	ZONE2=""
	if [ "${CSP}" == "aws" ]; then
		ZONE1=$(cat <<-END
		,
		"Zone": "${RegionNativeName[$INDEX,$REGION]}a"
		
END
		);
		ZONE2=$(cat <<-END
		,
		"Zone": "${RegionNativeName[$INDEX,$REGION]}b"
END
		);
	fi

	req=$(cat << EOF
	{
		"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
		"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
		"cidrBlock": "${CidrBlock}",
		"subnetInfoList": [ {
			"Name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"IPv4_CIDR": "${IPv4CIDR01}"
			${ZONE1}
		}, {
			"Name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}-01",
			"IPv4_CIDR": "${IPv4CIDR02}"
			${ZONE2}
		} ]
	}
EOF
	); echo ${req} | jq '.'

	resp=$(
        	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/vNet -H 'Content-Type: application/json' -d @- <<EOF
			${req}
EOF
	); echo ${resp} | jq '.'
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
		
		ResourceRegionNativeName=${RegionNativeName[$INDEX,$REGION]}

		CallTB

	fi
	
#}

#create_vNet
