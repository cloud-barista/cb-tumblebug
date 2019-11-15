#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
#	ID=`curl -sX GET http://$RESTSERVER:1024/publicip?connection_name=${NAME} |json_pp |grep "\"Name\" :" |awk '{print $3}' | head -n 1 |sed 's/"//g' |sed 's/,//g'`
#	curl -sX GET http://$RESTSERVER:1024/publicip/${ID}?connection_name=${NAME} |json_pp &
#done

TB_SPEC_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/spec | jq -r '.spec[].id'`
#echo $TB_SPEC_IDS | json_pp

if [ -n "$TB_SPEC_IDS" ]
then
        #TB_SPEC_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/spec | jq -r '.spec[].id'`
        for TB_SPEC_ID in ${TB_SPEC_IDS}
        do
                echo ....Get ${TB_SPEC_ID} ...
                curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/spec/${TB_SPEC_ID} | json_pp
        done
else
        echo ....no specs found
fi
