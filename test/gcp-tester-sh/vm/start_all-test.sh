#!/bin/bash
source ../setup.env

IMG_ID=ubuntu-minimal-1804-bionic-v20191024
IMG_ID=projects/ubuntu-os-cloud/global/images/ubuntu-minimal-1804-bionic-v20191024

num=0
for NAME in "${CONNECT_NAMES[@]}"
do
        echo ========================== $NAME
#        VNET_ID=cb-vnet
#        PIP_ID=publicipt${num}-powerkim
#        SG_ID1=security01-powerkim
        #echo ${VNET_ID}, ${PIP_ID}, ${SG_ID}, ${VNIC_ID}

#############################################################################################################################################

        TB_NETWORK_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/network | jq -r '.network[].id'`
        #echo $TB_NETWORK_IDS | json_pp

        if [ -n "$TB_NETWORK_IDS" ]
        then
                #TB_NETWORK_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/network | jq -r '.network[].id'`
                for TB_NETWORK_ID in ${TB_NETWORK_IDS}
                do
                        echo ....Get ${TB_NETWORK_ID} ...
                        NETWORKS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/network/${TB_NETWORK_ID} | jq -r '.connectionName'`
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

        TB_PUBLICIP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp | jq -r '.publicIp[].id'`
        #echo $TB_PUBLICIP_IDS | json_pp

        if [ -n "$TB_PUBLICIP_IDS" ]
        then
                #TB_PUBLICIP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp | jq -r '.publicIp[].id'`
                for TB_PUBLICIP_ID in ${TB_PUBLICIP_IDS}
                do
                        echo ....Get ${TB_PUBLICIP_ID} ...
                        PIPS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp/${TB_PUBLICIP_ID} | jq -r '.connectionName'`
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

TB_SECURITYGROUP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup | jq -r '.securityGroup[].id'`
        #echo $TB_SECURITYGROUP_IDS | json_pp

        if [ -n "$TB_SECURITYGROUP_IDS" ]
        then
                #TB_SECURITYGROUP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup | jq -r '.securityGroup[].id'`
                for TB_SECURITYGROUP_ID in ${TB_SECURITYGROUP_IDS}
                do
                        echo ....Get ${TB_SECURITYGROUP_ID} ...
                        SGS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/securityGroup/${TB_SECURITYGROUP_ID} | jq -r '.connectionName'`
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

        TB_SSHKEY_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey | jq -r '.sshKey[].id'`
        #echo $TB_SSHKEY_IDS | json_pp

        if [ -n "$TB_SSHKEY_IDS" ]
        then
                #TB_SSHKEY_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey | jq -r '.sshKey[].id'`
                for TB_SSHKEY_ID in ${TB_SSHKEY_IDS}
                do
                        echo ....Get ${TB_SSHKEY_ID} ...
                        SSHKEYS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/sshKey/${TB_SSHKEY_ID} | jq -r '.connectionName'`
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

        TB_SPEC_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/spec | jq -r '.spec[].id'`
        #echo $TB_SPEC_IDS | json_pp

        if [ -n "$TB_SPEC_IDS" ]
        then
                #TB_SPEC_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/spec | jq -r '.spec[].id'`
                for TB_SPEC_ID in ${TB_SPEC_IDS}
                do
                        echo ....Get ${TB_SPEC_ID} ...
                        SPECS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/spec/${TB_SPEC_ID} | jq -r '.connectionName'`
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

        TB_IMAGE_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/image | jq -r '.image[].id'`
        #echo $TB_IMAGE_IDS | json_pp

        if [ -n "$TB_IMAGE_IDS" ]
        then
                #TB_IMAGE_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/image | jq -r '.image[].id'`
                for TB_IMAGE_ID in ${TB_IMAGE_IDS}
                do
                        echo ....Get ${TB_IMAGE_ID} ...
                        IMAGES_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/image/${TB_IMAGE_ID} | jq -r '.connectionName'`
                        if [ "$IMAGES_CONN_NAME" == "$NAME" ]
                        then
                                IMAGE_ID=$TB_IMAGE_ID
                                echo IMAGE_ID: $IMAGE_ID
                                break
                        fi
                done
        else
                echo ....no images found
                exit 1
        fi

#############################################################################################################################################

#        curl -sX POST http://$RESTSERVER:1024/vm?connection_name=${NAME} -H 'Content-Type: application/json' -d '{
#            "VMName": "vm-powerkim01",
#                "ImageId": "'${IMG_ID}'",
#                "VirtualNetworkId": "'${VNET_ID}'",
#		"NetworkInterfaceId": "",
#                "PublicIPId": "'${PIP_ID}'",
#            "SecurityGroupIds": [ "'${SG_ID1}'" ],
#                "VMSpecId": "f1-micro",
#                 "KeyPairName": "mcb-keypair-powerkim",
#                "VMUserId": "cb-user"
#        }' |json_pp &

        if [ $num == 0 ]
        then
                curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/mcis -H 'Content-Type: application/json' -d '{
            "name": "mcis-t01",
            "description": "Test description",
            "vm_req": [
                {
                    "name": "jhseo-shooter-'$num'",
                    "config_name": "'$NAME'",
                    "spec_id": "'$SPEC_ID'",
                    "image_id": "'$IMAGE_ID'",
                    "vnet_id": "'$VNET_ID'",
                    "vnic_id": "",
                    "public_ip_id": "'$PIP_ID'",
                    "security_group_ids": [
                                "'$SG_ID'"
                        ],
                    "ssh_key_id": "'$SSHKEY_ID'",
                    "description": "description",
                    "vm_access_id": "cb-user",
                    "vm_access_passwd": ""
                }
            ]
        }' | json_pp

        else
                MCIS_ID=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/mcis | jq -r '.mcis[].id'`
                curl -sX POST http://$TUMBLEBUG_IP:1323/ns/$NS_ID/mcis/$MCIS_ID/vm -H 'Content-Type: application/json' -d '{
                "name": "jhseo-shooter-'$num'",
                    "config_name": "'$NAME'",
                    "spec_id": "'$SPEC_ID'",
                    "image_id": "'$IMAGE_ID'",
                    "vnet_id": "'$VNET_ID'",
                    "vnic_id": "",
                    "public_ip_id": "'$PIP_ID'",
                    "security_group_ids": [
                                "'$SG_ID'"
                        ],
                    "ssh_key_id": "'$SSHKEY_ID'",
                    "description": "description",
                    "vm_access_id": "cb-user",
                    "vm_access_passwd": ""
                }' | json_pp

        fi

        num=`expr $num + 1`
done

