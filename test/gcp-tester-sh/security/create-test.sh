#!/bin/bash
source ../setup.env

num=0
for NAME in "${CONNECT_NAMES[@]}"
do
	#NAME=${CONNECT_NAMES[0]}
#        curl -sX POST http://$RESTSERVER:1024/securitygroup?connection_name=${NAME} -H 'Content-Type: application/json' -d '{ "Name": "security01-powerkim", "SecurityRules": [ {"FromPort": "1", "ToPort" : "65535", "IPProtocol" : "tcp", "Direction" : "inbound"} ] }' |json_pp &
	curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup -H 'Content-Type: application/json' -d '{"connectionName":"'$NAME'",  "cspSecurityGroupName": "jhseo-test'$num'", "firewallRules": [ {"FromPort": "1", "ToPort" : "65535", "IPProtocol" : "tcp", "Direction" : "inbound"} ] }' | json_pp &

	num=`expr $num + 1`

done
