#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
#	ID=`curl -sX GET http://$RESTSERVER:1024/securitygroup?connection_name=${NAME} |json_pp |grep "\"Id\" :" |awk '{print $3}' |awk '{if(NR==2) print $1}' |sed 's/"//g' |sed 's/,//g'`
#	curl -sX DELETE http://$RESTSERVER:1024/securitygroup/${ID}?connection_name=${NAME}
#done

TB_SECURITYGROUP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
#echo $TB_SECURITYGROUP_IDS | json_pp

if [ "$TB_SECURITYGROUP_IDS" != "" ]
then
        TB_SECURITYGROUP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
        for TB_SECURITYGROUP_ID in ${TB_SECURITYGROUP_IDS}
        do
                echo ....Delete ${TB_SECURITYGROUP_ID} ...
                curl -sX DELETE http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup/${TB_SECURITYGROUP_ID} | json_pp
        done
else
        echo ....no securityGroups found
fi
