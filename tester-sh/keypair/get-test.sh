#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
#	curl -sX GET http://$RESTSERVER:1024/keypair/mcb-keypair-powerkim?connection_name=${NAME} |json_pp &
#done

TB_SSHKEY_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
#echo $TB_SSHKEY_IDS | json_pp

if [ "$TB_SSHKEY_IDS" != "" ]
then
        TB_SSHKEY_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
        for TB_SSHKEY_ID in ${TB_SSHKEY_IDS}
        do
                echo ....Get ${TB_SSHKEY_ID} ...
                curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey/${TB_SSHKEY_ID} | json_pp
        done
else
        echo ....no sshKeys found
fi
