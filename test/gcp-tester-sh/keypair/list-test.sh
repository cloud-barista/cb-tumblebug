#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
#	NAME=${CONNECT_NAMES[0]}
	curl -sX GET http://$TUMBLEBUG_IP:1323/ns/${NS_ID}/resources/sshKey |json_pp &
#done

