#!/bin/bash
source ../setup.env

for NAME in "${CONNECT_NAMES[@]}"
do
	curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup -H 'Content-Type: application/json' -d "{\"connectionName\":\"$NAME\",  \"cspSecurityGroupName\": \"jhseo-test\", \"firewallRules\": [ {\"FromPort\": \"20\", \"ToPort\" : \"200\", \"IPProtocol\" : \"tcp\", \"Direction\" : \"inbound\"} ] }" | json_pp &
done
