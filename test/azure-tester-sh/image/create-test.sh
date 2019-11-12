#!/bin/bash
source ../setup.env

num=0
for NAME in "${CONNECT_NAMES[@]}"
do
        curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/image?action=registerWithInfo -H 'Content-Type: application/json' -d '{"connectionName":"'$NAME'", 
        "cspImageId": "Canonical:UbuntuServer:18.04-LTS:latest",
        "cspImageName": "Canonical:UbuntuServer:18.04-LTS:latest"
        }' | json_pp

        num=`expr $num + 1`
done
