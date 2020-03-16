#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
#	ID=`curl -sX GET http://$RESTSERVER:1024/publicip?connection_name=${NAME} |json_pp |grep "\"Name\" :" |awk '{print $3}' | head -n 1 |sed 's/"//g' |sed 's/,//g'`
#	curl -sX GET http://$RESTSERVER:1024/publicip/${ID}?connection_name=${NAME} |json_pp &
#done

TB_PUBLICIP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp | jq -r '.publicIp[].id'`
#echo $TB_PUBLICIP_IDS | json_pp

if [ -n "$TB_PUBLICIP_IDS" ]
then
        #TB_PUBLICIP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp | jq -r '.publicIp[].id'`
        for TB_PUBLICIP_ID in ${TB_PUBLICIP_IDS}
        do
                echo ....Get ${TB_PUBLICIP_ID} ...
                curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp/${TB_PUBLICIP_ID} | json_pp
        done
else
        echo ....no publicIps found
fi
