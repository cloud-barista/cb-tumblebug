#!/bin/bash
source ../setup.env

for NAME in "${CONNECT_NAMES[@]}"
do
	curl -sX POST http://$RESTSERVER:1024/securitygroup?connection_name=${NAME} -H 'Content-Type: application/json' -d '{ "Name": "security01-powerkim", "SecurityRules": [ {"FromPort": "20", "ToPort" : "200", "IPProtocol" : "tcp", "Direction" : "inbound"} ] }' |json_pp &
done
