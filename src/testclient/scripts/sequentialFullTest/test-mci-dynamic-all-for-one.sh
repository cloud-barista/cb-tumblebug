#!/bin/bash

echo "####################################################################"
echo "## test-infra-dynamic-all.sh (parameters: -x (create or delete) -y numVM)"
echo "####################################################################"

source ../init.sh

# create or delete
option=${OPTION01}
nodeGroupSizeInput=${OPTION02:-1}

PRINT="index,infraName,connectionName,specId,image,nodeGroupSize,startTime,endTime,elapsedTime,option"
echo "${PRINT}" >./infraTest-$option.csv

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

imageId="ubuntu22.04"

MainInfraName="allforone"
infraName=$MainInfraName

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
                echo "[$i] connection: $connectionName / specId: $specId / image: $imageId / replica: $nodeGroupSizeInput "
            elif [ "${option}" == "delete" ]; then
                echo "[$i] infraName: $infraName (connection: $connectionName specId: $specId) "
            fi
            ((i++))

            if [ $i -ge $maxIterations ]; then
                break
            fi
        }
done

echo
echo "[Test] will $option Infras using all common Specs sequentially"
echo "[options] Operation: $option , infraSize: $nodeGroupSizeInput , fileName: infraTest-$option.csv"
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
        vmArray=$(echo "$vmArray" | jq --arg imageId "$imageId" --arg specId "$specId" --arg nodeGroupSizeInput "$nodeGroupSizeInput"  '. + [{"imageId": $imageId, "specIdspecId, "nodeGroupSize": $nodeGroupSizeInput}]')
        ((i++))

        # Break the loop when max iterations are reached
        if [ $i -ge $maxIterations ]; then
            break
        fi
    }
done

# Construct the request body with the accumulated JSON array
installMonAgent="no"
requestBody=$(jq -n --arg name "$infraName" --arg installMonAgent "$installMonAgent" --argjson vm "$vmArray" '{name: $name, installMonAgent: $installMonAgent , vm: $vm}')
echo "requestBody: $requestBody"

if [ "${option}" == "delete" ]; then
    echo "Terminate and Delete [$infraName]"
    curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/infra/${infraName}?option=terminate | jq '.'
elif [ "${option}" == "create" ]; then
    echo "Provisioning MC-Infra dynamically: [$infraName]"
    response=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/infraDynamic -H 'Content-Type: application/json' -d "$requestBody")
    #echo "${response}" | jq '.'


    infraResponse=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/infra/${infraName})


    echo -e "${BOLD}Table: All VMs in the Infra : ${infraName}${NC} ${BLUE} ${BOLD}"
    echo "$infraResponse" |
        jq '.vm | sort_by(.connectionName)' |
        jq -r '(["CloudRegion", "ID(TB)", "Status", "PublicIP", "PrivateIP", "DateTime(Created)"] | 
            (., map(length*"-"))), 
            (.[] | [.connectionName, .id, .status, .publicIP, .privateIP, .createdTime]) | @tsv' |
        column -t
    echo -e "${NC}"

    echo ""

    echo -e "${BOLD}MC-Infra: ${infraName} Status Summary ${NC}"
    echo "$infraResponse" | jq '.status'

fi

echo "Done!"

duration=$SECONDS
printElapsed $@
echo ""

