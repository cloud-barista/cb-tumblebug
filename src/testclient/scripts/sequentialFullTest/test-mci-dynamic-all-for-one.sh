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
installMonAgent="no"
label="DynamicVM"

echo 

specList=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/system/resources/spec)
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
            mciName=$specId

            if [ "${option}" == "create" ]; then
                echo "[$i] connection: $connectionName / specId: $specId / image: $image / replica: $subGroupSize "
            elif [ "${option}" == "delete" ]; then
                echo "[$i] mciName: $mciName / replica: $subGroupSize "
            fi
            ((i++))
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

specList=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/system/resources/spec)
specArray=$(jq -r '.spec' <<<"$specList")

MainMciName="allforone"
mciName=$MainMciName
firstSpecId=""
if [ "${option}" == "delete" ]; then
    echo "Terminate and Delete [$mciName]"
    curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/mci/${mciName}?option=terminate | jq '.'
elif [ "${option}" == "create" ]; then

    i=0
    for row in $(echo "${specArray}" | jq -r '.[] | @base64'); do
    {
        if [ "${i}" == "0" ]; then
            connectionName=$(_jq '.connectionName')
            specId=$(_jq '.id')
            firstSpecId=$specId
            rootDiskType=$(_jq '.rootDiskType')
            rootDiskSize=$(_jq '.rootDiskSize')
            image="ubuntu18.04"
            subGroupSize=$subGroupSizeInput

            mciName=$MainMciName

            echo
            echo "mciName: $mciName   specId: $specId   image: $image   connectionName: $connectionName   rootDiskType: $rootDiskType   rootDiskSize: $rootDiskSize  subGroupSize: $subGroupSize "

            startTime=$SECONDS

            echo "Creat MCI dynamic [$mciName]"
            VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/mciDynamic -H 'Content-Type: application/json' -d @- <<EOF
            {
                "name": "${mciName}",
                "description": "${description}",
                "installMonAgent": "${installMonAgent}",
                "label": "${label}",
                "systemLabel": "Managed-by-Tumblebug",
                "vm": [ {
                        "name": "${specId}",
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

            echo "${VAR1}" | jq '.'


            endTime=$SECONDS
            elapsedTime=$(($endTime-$startTime))

            PRINT="${i},${mciName},${connectionName},${specId},${image},${subGroupSize},${startTime},${endTime},${elapsedTime},${option}"
            echo "$PRINT"
            echo "$PRINT" >>./mciTest-$option.csv

            echo "[$i] Elapsed time: $elapsedTime s"
            ((i++))

            _jq() {
                echo ${row} | base64 --decode | jq -r ${1}
            }
            break
        fi

    }
    done

    i=0
    for row in $(echo "${specArray}" | jq -r '.[] | @base64'); do
        {
            _jq() {
                echo ${row} | base64 --decode | jq -r ${1}
            }

            if [ "${firstSpecId}" != "$(_jq '.id')" ]; then

                connectionName=$(_jq '.connectionName')

                specId=$(_jq '.id')
                rootDiskType=$(_jq '.rootDiskType')
                rootDiskSize=$(_jq '.rootDiskSize')
                image="ubuntu18.04"
                subGroupSize=$subGroupSizeInput

                echo
                echo "mciName: $mciName   specId: $specId   image: $image   connectionName: $connectionName   rootDiskType: $rootDiskType   rootDiskSize: $rootDiskSize  subGroupSize: $subGroupSize "
                sleepDuration=$((1 + $RANDOM% 1000))
                echo "sleepDuration: $sleepDuration"
                #sleep $sleepDuration

                startTime=$SECONDS

                    echo "Creat VM dynamic [$mciName]"
                    VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/mci/$mciName/vmDynamic -H 'Content-Type: application/json' -d @- <<EOF
                    {
                        "name": "${specId}",
                        "commonImage": "${image}",
                        "commonSpec": "${specId}",
                        "rootDiskType": "${rootDiskType}",
                        "rootDiskSize": "${rootDiskSize}",
                        "subGroupSize": "${subGroupSize}"
                    }
EOF
                    )

                    echo "${VAR1}" | jq '.'

                endTime=$SECONDS
                elapsedTime=$(($endTime-$startTime))

                PRINT="${i},${mciName},${connectionName},${specId},${image},${subGroupSize},${startTime},${endTime},${elapsedTime},${option}"
                echo "$PRINT"
                echo "$PRINT" >>./mciTest-$option.csv

                echo "[$i] Elapsed time: $elapsedTime s"
                ((i++))
            fi
        } #&

    done
    #wait


fi



echo "Done!"
duration=$SECONDS
printElapsed $@
echo ""

