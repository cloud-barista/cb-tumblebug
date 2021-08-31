#!/bin/bash

echo "####################################################################"
echo "## update-dns-for-mcis-ip (parameters: -x HostedZoneID -y RecordName)"
echo "####################################################################"

SECONDS=0

source ../init.sh

HostedZoneID=${OPTION01}
RecordName=${OPTION02:-conf.cloud-barista.org}

COMMAND=aws
if ! command -v $COMMAND &> /dev/null
then
    echo "AWS CLI (to control Route 53 DNS service) is required for this script"
	echo "Please check following instruction to install and configure AWS CLI(v2)"
	echo "- https://docs.aws.amazon.com/ko_kr/cli/latest/userguide/install-cliv2-linux.html"
	echo "- https://docs.aws.amazon.com/ko_kr/cli/latest/userguide/cli-configure-files.html"
    exit
fi

if [ "${INDEX}" == "0" ]; then
	MCISID=${MCISPREFIX}-${POSTFIX}
fi

if [ -z "$HostedZoneID" ]; then
	echo "[Warning] Provide your HostedZones.Id (ex: /hostedzone/XXXX9210PL5XXXOY9B7T) to -x parameter"
	echo "Please retrieve HostedZones.Id from AWS Routh 53"
	echo "You can provide RecordName to -x parameter"
	echo `aws route53 list-hosted-zones` | jq ''
	exit
fi

MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?action=status)
VMARRAY=$(jq -r '.status.vm' <<<"$MCISINFO")

echo "VMARRAY: $VMARRAY"

PublicIP=""
VMID=""

aws route53 list-hosted-zones | jq ''

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	VMID=$(_jq '.id')
	PublicIP=$(_jq '.publicIp')

	# ref: http://www.scalingbits.com/aws/dnsfailover/changehostnameentries
	Result=$(aws route53 change-resource-record-sets --hosted-zone-id $HostedZoneID --change-batch '{
    "Comment": "Update record to reflect new IP address for a system ",
    "Changes": [
        {
            "Action": "UPSERT",
            "ResourceRecordSet": {
                "Name": "'${RecordName}'",
                "Type": "A",
                "TTL": 100,
                "ResourceRecords": [
                    {
                        "Value": "'${PublicIP}'"
                    }
                ]
            }
        }
    ]
	}')
	echo $Result | jq ''
	ChangeStatus=$(echo $Result | jq '.ChangeInfo.Status | @base64' | base64 -di)
	ChangeInfoID=$(echo $Result | jq '.ChangeInfo.Id | @base64' | base64 -di)
	echo "$ChangeStatus $ChangeInfoID"

	dozing 30

	for ((try = 1; try <= 20; try++)); do
		Result=$(aws route53 get-change --id ${ChangeInfoID})
		echo $Result | jq ''
		ChangeStatus=$(echo $Result | jq '.ChangeInfo.Status | @base64' | base64 -di)
		ChangeInfoID=$(echo $Result | jq '.ChangeInfo.Id | @base64' | base64 -di)
		echo "$ChangeStatus $ChangeInfoID"
		if [ ${ChangeStatus} = "INSYNC" ]; then
			break
		else
			printf "[$try : Record update is working on].."
			dozing 3
		fi
	done

done

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

echo "[DNS is ready]"
echo "[DNS Record: ${RecordName}, Assigned IP: ${PublicIP}]"
echo ""
