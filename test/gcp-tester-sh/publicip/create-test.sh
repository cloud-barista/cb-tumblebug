#!/bin/bash
source ../setup.env


num=0
for NAME in "${CONNECT_NAMES[@]}"
do
#        curl -sX POST http://$RESTSERVER:1024/publicip?connection_name=${NAME} -H 'Content-Type: application/json' -d '{ "Name": "publicipt'${num}'-powerkim" }' |json_pp &
	curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp -H 'Content-Type: application/json' -d '{"connectionName":"'$NAME'", "cspPublicIpName":"jhseo-shooter'${num}'"}' | json_pp

	num=`expr $num + 1`
done

