#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
	curl -sX GET http://$TUMBLEBUG_IP:1323/ns/${NS_ID}/resources/network | json_pp &
#done
