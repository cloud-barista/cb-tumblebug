#!/bin/bash
source ../setup.env

num=0
for NAME in "${CONNECT_NAMES[@]}"
do
	echo ========================== $NAME
	#VNET_ID=/subscriptions/f1548292-2be3-4acd-84a4-6df079160846/resourceGroups/CB-GROUP-POWERKIM/providers/Microsoft.Network/virtualNetworks/CB-VNet
	VNET_ID=CB-VNet-powerkim
	PIP_ID=publicipt01-powerkim
	SG_ID1=security01-powerkim
	#echo ${VNET_ID}, ${PIP_ID}, ${SG_ID}, ${VNIC_ID}

	curl -sX POST http://$RESTSERVER:1024/vm?connection_name=${NAME} -H 'Content-Type: application/json' -d '{
	    "VMName": "vm-powerkim01",
		"ImageId": "'${IMG_IDS[num]}'",
		"VirtualNetworkId": "'${VNET_ID}'",
		"PublicIPId": "'${PIP_ID}'",
	    "SecurityGroupIds": [ "'${SG_ID1}'" ],
		"VMSpecId": "Standard_B1ls",
		 "KeyPairName": "mcb-keypair-powerkim",
		"VMUserId": "cb-user"
	}' |json_pp &


	num=`expr $num + 1`
done
