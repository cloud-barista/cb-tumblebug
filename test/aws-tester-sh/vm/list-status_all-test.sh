#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
#	echo ========================== $NAME
#	curl -sX GET http://$RESTSERVER:1024/vmstatus?connection_name=$NAME |json_pp
#done

MCIS_ID=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/mcis | json_pp | grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/mcis/$MCIS_ID?action=status | json_pp
