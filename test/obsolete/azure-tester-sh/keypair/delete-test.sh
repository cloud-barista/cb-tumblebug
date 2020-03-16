#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
#	NAME=${CONNECT_NAMES[0]}
#	curl -sX DELETE http://$RESTSERVER:1024/keypair/mcb-keypair-powerkim?connection_name=${NAME} |json_pp
#done

TB_SSHKEY_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey | jq -r '.sshKey[].id'`

if [ -n "$TB_SSHKEY_IDS" ]
then
        #TB_SSHKEY_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey | jq -r '.sshKey[].id'`
        for TB_SSHKEY_ID in ${TB_SSHKEY_IDS}
        do
                echo ....Delete ${TB_SSHKEY_ID} ...
                curl -sX DELETE http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey/${TB_SSHKEY_ID}
        done
else
        echo ....no sshKeys found
fi
