#!/bin/bash
source ../setup.env

for NAME in "${CONNECT_NAMES[@]}"
do
	curl -sX POST http://$RESTSERVER:1024/publicip?connection_name=${NAME} -H 'Content-Type: application/json' -d '{ "Name": "publicipt01-powerkim" }' |json_pp &
done
