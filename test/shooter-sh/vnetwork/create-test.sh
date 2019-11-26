#!/bin/bash
source ../setup.env

for NAME in "${CONNECT_NAMES[@]}"
do
#        curl -sX POST http://$RESTSERVER:1024/vnetwork?connection_name=${NAME} -H 'Content-Type: application/json' -d '{"Name":"cb-vnet"}' |json_pp &
	curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/network -H 'Content-Type: application/json' -d '{"connectionName":"'$NAME'", "cspNetworkName":"jhseo-test"}' | json_pp 
#	sleep 10
done

