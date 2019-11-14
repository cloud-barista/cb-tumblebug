#!/bin/bash
source ../setup.env

IMG_ID=ubuntu-minimal-1804-bionic-v20191024
IMG_ID=projects/ubuntu-os-cloud/global/images/ubuntu-minimal-1804-bionic-v20191024

num=0
for NAME in "${CONNECT_NAMES[@]}"
do
        echo ========================== $NAME
        VNET_ID=cb-vnet
        PIP_ID=publicipt${num}-powerkim
        SG_ID1=security01-powerkim
        #echo ${VNET_ID}, ${PIP_ID}, ${SG_ID}, ${VNIC_ID}

        curl -sX POST http://$RESTSERVER:1024/vm?connection_name=${NAME} -H 'Content-Type: application/json' -d '{
            "VMName": "vm-powerkim01",
                "ImageId": "'${IMG_ID}'",
                "VirtualNetworkId": "'${VNET_ID}'",
		"NetworkInterfaceId": "",
                "PublicIPId": "'${PIP_ID}'",
            "SecurityGroupIds": [ "'${SG_ID1}'" ],
                "VMSpecId": "f1-micro",
                 "KeyPairName": "mcb-keypair-powerkim",
                "VMUserId": "cb-user"
        }' |json_pp &


        num=`expr $num + 1`
done

