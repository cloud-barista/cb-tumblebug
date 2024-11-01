#!/bin/bash

echo "####################################################################"
echo "## test-mci-dynamic-all.sh (parameters: -x (create or delete) -y numVM)"
echo "####################################################################"

source ../init.sh

# create or delete
option=${OPTION01}
subGroupSizeInput=${OPTION02:-1}

PRINT="index,mciName,connectionName,specId,image,subGroupSize,startTime,endTime,elapsedTime,option"
echo "${PRINT}" >./mciTest-$option.csv

description="Made in CB-TB"
label="DynamicVM"

echo 
maxIterations=30

specIdArray=(
aws+ap-northeast-1+t2.small
aws+ap-northeast-2+t2.small
aws+ap-northeast-3+t2.small
aws+ap-south-1+t2.small
aws+ap-southeast-1+t2.small
aws+ap-southeast-2+t2.small
aws+ca-central-1+t2.small
aws+eu-west-1+t2.small
aws+sa-east-1+t2.small
aws+us-west-1+t2.small
aws+us-west-2+t2.small
gcp+asia-east1+g1-small
gcp+asia-east2+g1-small
gcp+asia-northeast1+g1-small
gcp+asia-northeast2+g1-small
gcp+asia-northeast3+g1-small
gcp+asia-south1+g1-small
gcp+asia-southeast1+g1-small
gcp+asia-southeast2+g1-small
gcp+australia-southeast1+g1-small
gcp+europe-central2+g1-small
azure+australiacentral+standard_b1s
azure+australiaeast+standard_b1s
azure+canadacentral+standard_b1s
azure+centralus+standard_b1s
azure+eastus2+standard_b1s
azure+japaneast+standard_b1s
azure+ukwest+standard_b1s
azure+koreacentral+standard_b1ms
azure+koreasouth+standard_b1ms
)

specArray="[]"
for specId in "${specIdArray[@]}"; do
  spec=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/system/resources/spec/${specId})
  specArray=$(echo "$specArray" | jq --argjson spec "$spec" '. + [$spec]')
  sleep 0.1
done

echo "Number of specs: $(echo "$specArray" | jq length)"

commonImage="ubuntu22.04"

MainMciName="allforone"
mciName=$MainMciName

i=0

for row in $(echo "${specArray}" | jq -r '.[] | @base64'); do
        {
            _jq() {
                echo ${row} | base64 --decode | jq -r ${1}
            }
            connectionName=$(_jq '.connectionName')
            specId=$(_jq '.id')
            rootDiskType=$(_jq '.rootDiskType')
            rootDiskSize=$(_jq '.rootDiskSize')

            if [ "${option}" == "create" ]; then
                echo "[$i] connection: $connectionName / specId: $specId / image: $commonImage / replica: $subGroupSizeInput "
            elif [ "${option}" == "delete" ]; then
                echo "[$i] mciName: $mciName (connection: $connectionName specId: $specId) "
            fi
            ((i++))

            if [ $i -ge $maxIterations ]; then
                break
            fi
        }
done

echo
echo "[Test] will $option MCIs using all common Specs sequentially"
echo "[options] Operation: $option , mciSize: $subGroupSizeInput , fileName: mciTest-$option.csv"
echo

while true; do
    read -p 'Confirm the above configuration. Do you want to proceed ? (y/n) : ' CHECKPROCEED
    echo -e "${NC}"
    case $CHECKPROCEED in
        [Yy]* ) 
            break
            ;;
        [Nn]* ) 
            echo
            echo "Cancel [$0 $@]"
            echo "See you soon. :)"
            echo
            exit 1
            ;;
        * ) 
            echo "Please answer yes or no.";;
    esac
done

SECONDS=0

# Initialize an empty array correctly
vmArray=$(jq -n '[]')  
i=0

for row in $(echo "${specArray}" | jq -r '.[] | @base64'); do
    {
        _jq() {
            echo ${row} | base64 --decode | jq -r ${1}
        }
        specId=$(_jq '.id')
        echo "specId: $specId"

        # Properly append to the JSON array
        vmArray=$(echo "$vmArray" | jq --arg commonImage "$commonImage" --arg specId "$specId" --arg subGroupSizeInput "$subGroupSizeInput"  '. + [{"commonImage": $commonImage, "commonSpec": $specId, "subGroupSize": $subGroupSizeInput}]')
        ((i++))

        # Break the loop when max iterations are reached
        if [ $i -ge $maxIterations ]; then
            break
        fi
    }
done

# Construct the request body with the accumulated JSON array
installMonAgent="no"
requestBody=$(jq -n --arg name "$mciName" --arg installMonAgent "$installMonAgent" --argjson vm "$vmArray" '{name: $name, installMonAgent: $installMonAgent , vm: $vm}')
echo "requestBody: $requestBody"

if [ "${option}" == "delete" ]; then
    echo "Terminate and Delete [$mciName]"
    curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/mci/${mciName}?option=terminate | jq '.'
elif [ "${option}" == "create" ]; then
    echo "Provisioning MC-Infra dynamically: [$mciName]"
    response=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/mciDynamic -H 'Content-Type: application/json' -d "$requestBody")
    #echo "${response}" | jq '.'


    mciResponse=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci/${mciName})


    echo -e "${BOLD}Table: All VMs in the MCI : ${mciName}${NC} ${BLUE} ${BOLD}"
    echo "$mciResponse" |
        jq '.vm | sort_by(.connectionName)' |
        jq -r '(["CloudRegion", "ID(TB)", "Status", "PublicIP", "PrivateIP", "DateTime(Created)"] | 
            (., map(length*"-"))), 
            (.[] | [.connectionName, .id, .status, .publicIP, .privateIP, .createdTime]) | @tsv' |
        column -t
    echo -e "${NC}"

    echo ""

    echo -e "${BOLD}MC-Infra: ${mciName} Status Summary ${NC}"
    echo "$mciResponse" | jq '.status'

fi

echo "Done!"

duration=$SECONDS
printElapsed $@
echo ""

