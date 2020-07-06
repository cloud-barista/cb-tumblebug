#!/bin/bash
source ../setup.env

for NAME in "${CONNECT_NAMES[@]}"
do
	ID=CB-VNet-powerkim
	curl -H "${AUTH}" -sX GET http://$RESTSERVER:1024/vpc/${ID}?connection_name=${NAME} |json_pp &
done
