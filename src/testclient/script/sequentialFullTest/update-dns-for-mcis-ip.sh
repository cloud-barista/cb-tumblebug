#!/bin/bash

SECONDS=0

echo "[Check jq package (if not, install)]"
if ! dpkg-query -W -f='${Status}' jq | grep "ok installed"; then sudo apt install -y jq; fi

COMMAND=aws
if ! command -v $COMMAND &> /dev/null
then
    echo "AWS CLI (to control Route 53 DNS service) is required for this script"
	echo "Please check following instruction to install and configure AWS CLI(v2)"
	echo "- https://docs.aws.amazon.com/ko_kr/cli/latest/userguide/install-cliv2-linux.html"
	echo "- https://docs.aws.amazon.com/ko_kr/cli/latest/userguide/cli-configure-files.html"
    exit
fi

TestSetFile=${4:-../testSet.env}

FILE=$TestSetFile
if [ ! -f "$FILE" ]; then
	echo "$FILE does not exist."
	exit
fi
source $TestSetFile
source ../conf.env
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

echo "####################################################################"
echo "## update-dns-for-mcis-ip "
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

MCISID=${CONN_CONFIG[$INDEX, $REGION]}-${POSTFIX}

HostedZoneID=${5}

RecordName=${6:-conf.cloud-barista.org}

if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${MCISPREFIX}-${POSTFIX}
fi

if [ -z "$HostedZoneID" ]; then
	echo "[Warning] Provide your HostedZones.Id (ex: /hostedzone/XXXX9210PL5XXXOY9B7T) to 5th parameter"
	echo "Please retrieve HostedZones.Id from AWS Routh 53"
	echo "You can provide RecordName to 6th parameter"
	echo `aws route53 list-hosted-zones` | jq ''
	exit
fi

MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${MCISID}?action=status)
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
	PublicIP=$(_jq '.public_ip')

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
echo "[CMD] $0"
echo "$(($duration / 60)) minutes and $(($duration % 60)) seconds elapsed."
echo ""

echo "[DNS is ready]"
echo "[DNS Record: ${RecordName}, Assigned IP: ${PublicIP}]"
echo ""
