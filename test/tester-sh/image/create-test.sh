#!/bin/bash
source ../setup.env

num=0
for NAME in "${CONNECT_NAMES[@]}"
do
	curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/image?action=registerWithId -H 'Content-Type: application/json' -d '{"connectionName":"'$NAME'", 
	"cspImageId": "'${IMG_IDS[num]}'"
	}' | json_pp

	num=`expr $num + 1`
done
