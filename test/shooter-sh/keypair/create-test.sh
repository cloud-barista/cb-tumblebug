#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
#        NAME=${CONNECT_NAMES[0]}
#	KEY=`curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d "{\"connectionName\":\"$NAME\", \"cspSshKeyName\":\"jhseo-test\"}" | json_pp | grep privateKey | sed 's/"privateKey" : "//g' | sed 's/-----",/-----/g' | sed 's/-----"/-----/g'`
#        echo -e ${KEY}
#        echo -e ${KEY} > ./${NAME}.key

#        chmod 600 ./${NAME}.key
#done

INDICES="0 1 2 3 4 5 6 7"
for i in $INDICES
do
	KEY=`curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey -H 'Content-Type: application/json' -d '{"connectionName":"'${CONNECT_NAMES[i]}'", "cspSshKeyName":"jhseo-test"}' | json_pp | grep privateKey | sed 's/"privateKey" : "//g' | sed 's/-----",/-----/g' | sed 's/-----"/-----/g'`
	echo -e ${KEY}
        echo -e ${KEY} > ./${CONNECT_NAMES[i]}.key

        chmod 600 ./${CONNECT_NAMES[i]}.key
done
