#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
        NAME=${CONNECT_NAMES[0]}
	KEY=`curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d "{\"connectionName\":\"$NAME\", \"cspSshKeyName\":\"jhseo-shooter\"}" | json_pp | grep privateKey | sed 's/"privateKey" : "//g' | sed 's/-----",/-----/g' | sed 's/-----"/-----/g'`
        echo -e ${KEY}
        echo -e ${KEY} > ./${NAME}.key

        chmod 600 ./${NAME}.key
#done

