#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
#	ID=security01-powerkim
#	curl -sX DELETE http://$RESTSERVER:1024/securitygroup/${ID}?connection_name=${NAME}
#done

TB_SECURITYGROUP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup | jq -r '.securityGroup[].id'`
#echo $TB_SECURITYGROUP_IDS | json_pp

if [ -n "$TB_SECURITYGROUP_IDS" ]
then
        #TB_SECURITYGROUP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup | jq -r '.securityGroup[].id'`
        for TB_SECURITYGROUP_ID in ${TB_SECURITYGROUP_IDS}
        do
                echo ....Delete ${TB_SECURITYGROUP_ID} ...
                curl -sX DELETE http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup/${TB_SECURITYGROUP_ID} | json_pp
        done
else
        echo ....no securityGroups found
fi
