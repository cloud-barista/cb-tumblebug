#!/bin/bash
source ../setup.env

for NAME in "${CONNECT_NAMES[@]}"
do
#        curl -H "${AUTH}" -sX POST http://$RESTSERVER:1024/vpc?connection_name=${NAME} -H 'Content-Type: application/json' -d '{"Name":"cb-vnet"}' |json_pp &
	curl -H "${AUTH}" -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/vNet -H 'Content-Type: application/json' -d '{"connectionName":"'$NAME'", "cspNetworkName":"jhseo-shooter"}' | json_pp 
	sleep 10
done

