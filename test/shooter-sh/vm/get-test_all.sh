#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
#	echo ========================== $NAME
#	curl -sX GET http://$RESTSERVER:1024/vm/vm-powerkim01?connection_name=$NAME |json_pp
#done

MCIS_ID=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/mcis | jq -r '.mcis[].id'`
curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/mcis/$MCIS_ID | json_pp
