#!/bin/bash
source ../setup.env

for NAME in "${CONNECT_NAMES[@]}"
do
	curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup -H 'Content-Type: application/json' -d '{"connectionName":"'$NAME'",  "cspSecurityGroupName": "jhseo-test", "firewallRules": [ {"FromPort": "0", "ToPort" : "65535", "IPProtocol" : "*", "Direction" : "inbound"} ] }' | json_pp &
done
