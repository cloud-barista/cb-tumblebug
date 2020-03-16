#!/bin/bash
source ../setup.env

for NAME in "${CONNECT_NAMES[@]}"
do
	curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp -H 'Content-Type: application/json' -d '{"connectionName":"'$NAME'", "cspPublicIpName":"jhseo-test"}' | json_pp
done
