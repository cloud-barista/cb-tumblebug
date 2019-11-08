#!/bin/bash
set -e
source ../setup.env

num=0
for NAME in "${CONNECT_NAMES[@]}"
do
	echo ========================== $NAME
#	VNET_ID=`curl -sX GET http://$RESTSERVER:1024/vnetwork?connection_name=${NAME} |json_pp |grep "\"Id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
#	PIP_ID=`curl -sX GET http://$RESTSERVER:1024/publicip?connection_name=${NAME} |json_pp |grep "\"Name\" :" |awk '{print $3}' | head -n 1 |sed 's/"//g' |sed 's/,//g'`
#	SG_ID1=` curl -sX GET http://$RESTSERVER:1024/securitygroup?connection_name=${NAME} |json_pp |grep "\"Id\" :" |awk '{print $3}' | head -n 1 |sed 's/"//g' |sed 's/,//g'`
#	SG_ID2=`curl -sX GET http://$RESTSERVER:1024/securitygroup?connection_name=${NAME} |json_pp |grep "\"Id\" :" |awk '{print $3}' |awk '{if(NR==2) print $1}' |sed 's/"//g' |sed 's/,//g'`
#	VNIC_ID=`curl -sX GET http://$RESTSERVER:1024/vnic?connection_name=${NAME} |json_pp |grep "eni" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`

#############################################################################################################################################

	TB_NETWORK_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/network | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
	#echo $TB_NETWORK_IDS | json_pp

	if [ "$TB_NETWORK_IDS" != "" ]
	then
		TB_NETWORK_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/network | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
		for TB_NETWORK_ID in ${TB_NETWORK_IDS}
		do
			echo ....Get ${TB_NETWORK_ID} ...
			NETWORKS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/network/${TB_NETWORK_ID} | json_pp | grep "\"connectionName\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
			if [ "$NETWORKS_CONN_NAME" == "$NAME" ]
			then
				VNET_ID=$TB_NETWORK_ID
				echo VNET_ID: $VNET_ID
				break
			fi
		done
	else
		echo ....no networks found. Exiting..
		exit 1
	fi

#############################################################################################################################################

	TB_PUBLICIP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
	#echo $TB_PUBLICIP_IDS | json_pp

	if [ "$TB_PUBLICIP_IDS" != "" ]
	then
		TB_PUBLICIP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp |json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
		for TB_PUBLICIP_ID in ${TB_PUBLICIP_IDS}
		do
			echo ....Get ${TB_PUBLICIP_ID} ...
			PIPS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp/${TB_PUBLICIP_ID} | json_pp | grep "\"connectionName\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
                        if [ "$PIPS_CONN_NAME" == "$NAME" ]
                        then
                                PIP_ID=$TB_PUBLICIP_ID
                                echo PIP_ID: $PIP_ID
				break
                        fi
		done
	else
		echo ....no publicIps found. Exiting..
		exit 1
	fi

#############################################################################################################################################

	TB_SECURITYGROUP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
	#echo $TB_SECURITYGROUP_IDS | json_pp

	if [ "$TB_SECURITYGROUP_IDS" != "" ]
	then
		TB_SECURITYGROUP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
		for TB_SECURITYGROUP_ID in ${TB_SECURITYGROUP_IDS}
		do
			echo ....Get ${TB_SECURITYGROUP_ID} ...
			SGS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup/${TB_SECURITYGROUP_ID} | json_pp | grep "\"connectionName\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
			if [ "$SGS_CONN_NAME" == "$NAME" ]
                        then
                                SG_ID=$TB_SECURITYGROUP_ID
                                echo SG_ID: $SG_ID
                                break
                        fi
		done
	else
		echo ....no securityGroups found. Exiting..
		exit 1
	fi

#############################################################################################################################################

	TB_SSHKEY_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
	#echo $TB_SSHKEY_IDS | json_pp

	if [ "$TB_SSHKEY_IDS" != "" ]
	then
		TB_SSHKEY_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
		for TB_SSHKEY_ID in ${TB_SSHKEY_IDS}
		do
			echo ....Get ${TB_SSHKEY_ID} ...
			SSHKEYS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey/${TB_SSHKEY_ID} | json_pp | grep "\"connectionName\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
			if [ "$SSHKEYS_CONN_NAME" == "$NAME" ]
                        then
                                SSHKEY_ID=$TB_SSHKEY_ID
                                echo SSHKEY_ID: $SSHKEY_ID
                                break
                        fi
		done
	else
		echo ....no sshKeys found. Exiting..
		exit 1
	fi

#############################################################################################################################################

	TB_SPEC_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/spec | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
	#echo $TB_SPEC_IDS | json_pp

	if [ "$TB_SPEC_IDS" != "" ]
	then
		TB_SPEC_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/spec |json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
		for TB_SPEC_ID in ${TB_SPEC_IDS}
		do
			echo ....Get ${TB_SPEC_ID} ...
			SPECS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/spec/${TB_SPEC_ID} | json_pp | grep "\"connectionName\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
			if [ "$SPECS_CONN_NAME" == "$NAME" ]
			then
				SPEC_ID=$TB_SPEC_ID
				echo SPEC_ID: $SPEC_ID
				break
			fi
		done
	else
		echo ....no specs found. Exiting..
		exit 1
	fi

#############################################################################################################################################

	#echo ${VNET_ID}, ${PIP_ID}, ${SG_ID}, ${VNIC_ID}, $SSHKEY_ID, $SPEC_ID

#	curl -sX POST http://$RESTSERVER:1024/vm?connection_name=${NAME} -H 'Content-Type: application/json' -d '{
#	    "VMName": "vm-powerkim01",
#		"ImageId": "'${IMG_IDS[num]}'",
#		"VirtualNetworkId": "'${VNET_ID}'",
#		"NetworkInterfaceId": "'${VNIC_ID}'",
#		"PublicIPId": "'${PIP_ID}'",
#	    "SecurityGroupIds": [
#		"'${SG_ID1}'",  "'${SG_ID2}'"
#	    ],
#		"VMSpecId": "t2.micro",
#		"KeyPairName": "mcb-keypair-powerkim",
#		"VMUserId": "",
#		"VMUserPasswd": ""
#	}' |json_pp &

if [ $num == 0 ]
then
	curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/mcis -H 'Content-Type: application/json' -d '{
    "name": "mcis-t01",
    "description": "Test description",
    "vm_req": [
    	{
            "name": "aws-shson-vm'$num'",
            "config_name": "'$NAME'",
            "spec_id": "'$SPEC_ID'",
            "image_id": "'$IMG_IDS[num]'",
            "vnet_id": "'$VNET_ID'",
            "vnic_id": "",
            "public_ip_id": "'$PIP_ID'",
            "security_group_ids": [
        		"'$SG_ID'"
    		],
            "ssh_key_id": "'$SSHKEY_ID'",
            "description": "description",
            "vm_access_id": "",
            "vm_access_passwd": ""
        }    
    ]
}' | json_pp

else

	MCIS_ID=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/mcis | json_pp | grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
	curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/mcis/$MCIS_ID/vm -H 'Content-Type: application/json' -d '{
	"name": "aws-shson-vm'$num'",
            "config_name": "'$NAME'",
            "spec_id": "'$SPEC_ID'",
            "image_id": "'$IMG_IDS[num]'",
            "vnet_id": "'$VNET_ID'",
            "vnic_id": "",
            "public_ip_id": "'$PIP_ID'",
            "security_group_ids": [
                        "'$SG_ID'"
                ],
            "ssh_key_id": "'$SSHKEY_ID'",
            "description": "description",
            "vm_access_id": "",
            "vm_access_passwd": ""
        }' | json_pp

fi

	num=`expr $num + 1`
done
