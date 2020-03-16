#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
	#NAME=${CONNECT_NAMES[0]}

        #curl -sX GET http://$RESTSERVER:1024/securitygroup?connection_name=${NAME} |json_pp &
	curl -sX GET http://$TUMBLEBUG_IP:1323/ns/${NS_ID}/resources/securityGroup | json_pp &
#done

