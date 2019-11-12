#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
	NAME=${CONNECT_NAMES[0]}
	KEY=`curl -sX POST http://$RESTSERVER:1024/keypair?connection_name=$NAME -H 'Content-Type: application/json' -d '{ "Name": "mcb-keypair-powerkim" }' |json_pp | grep PrivateKey |sed 's/"PrivateKey" : "//g' |sed 's/-----",/-----/g' |sed 's/-----"/-----/g'`
	echo -e ${KEY}
	echo -e ${KEY} > ./${NAME}.key

	chmod 600 ./${NAME}.key
#done
