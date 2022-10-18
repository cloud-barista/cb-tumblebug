#!/bin/bash

echo "####################################################################"
echo "## test-mcis-dynamic-all.sh (parameters: -x (create or delete) -y numVM)"
echo "####################################################################"



source ../init.sh

# create or delete
option=${OPTION01}
subGroupSizeInput=${OPTION02:-1}


PRINT="index,mcisName,connectionName,specId,image,subGroupSize,startTime,endTime,elapsedTime,option"
echo "${PRINT}" >./mcisTest-$option.csv


description="Made in CB-TB"
installMonAgent="no"
label="DynamicVM"

echo 

specList=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/system-purpose-common-ns/resources/spec)
specArray=$(jq -r '.spec' <<<"$specList")

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
            image="ubuntu18.04"
            subGroupSize=$subGroupSizeInput
            mcisName=$specId

            if [ "${option}" == "create" ]; then
                echo "[$i] connection: $connectionName / specId: $specId / image: $image / replica: $subGroupSize "
            elif [ "${option}" == "delete" ]; then
                echo "[$i] mcisName: $mcisName / replica: $subGroupSize "
            fi
            ((i++))
        }
done

echo
echo "[Test] will $option MCISs using all common Specs sequentially"
echo "[options] Operation: $option , mcisSize: $subGroupSizeInput , fileName: mcisTest-$option.csv"
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

specList=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/system-purpose-common-ns/resources/spec)
specArray=$(jq -r '.spec' <<<"$specList")

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
        image="ubuntu18.04"
        subGroupSize=$subGroupSizeInput
        mcisName=$specId

        echo
        echo "mcisName: $mcisName   specId: $specId   image: $image   connectionName: $connectionName   rootDiskType: $rootDiskType   rootDiskSize: $rootDiskSize  subGroupSize: $subGroupSize "
        sleepDuration=$((1 + RANDOM % 600))
        echo "sleepDuration: $sleepDuration"
        sleep $sleepDuration

        startTime=$SECONDS
        if [ "${option}" == "delete" ]; then
            echo "Terminate and Delete [$mcisName]"
            curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${mcisName}?option=terminate | jq ''


        elif [ "${option}" == "create" ]; then
            echo "Creat MCIS dynamic [$mcisName]"
            VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/mcisDynamic -H 'Content-Type: application/json' -d @- <<EOF
            {
                    "name": "${mcisName}",
                    "description": "${description}",
                    "installMonAgent": "${installMonAgent}",
                    "label": "${label}",
                    "systemLabel": "Managed-by-Tumblebug",
                    "vm": [ {
                            "commonImage": "${image}",
                            "commonSpec": "${specId}",
                            "rootDiskType": "${rootDiskType}",
                            "rootDiskSize": "${rootDiskSize}",
                            "subGroupSize": "${subGroupSize}"
                        }
                    ]
            }
EOF
            )

            echo "${VAR1}" | jq ''

        fi
        endTime=$SECONDS
        elapsedTime=$(($endTime-$startTime))

        PRINT="${i},${mcisName},${connectionName},${specId},${image},${subGroupSize},${startTime},${endTime},${elapsedTime},${option}"
        echo "$PRINT"
        echo "$PRINT" >>./mcisTest-$option.csv

        echo "[$i] Elapsed time: $elapsedTime s"
        ((i++))

    } &

done
wait



echo "Done!"
duration=$SECONDS
printElapsed $@
echo ""

