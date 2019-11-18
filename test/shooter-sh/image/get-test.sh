#!/bin/bash
source ../setup.env


#num=0
#for NAME in "${CONNECT_NAMES[@]}"
#do
#        curl -sX GET http://$RESTSERVER:1024/vmimage/${IMG_IDS[num]}?connection_name=${NAME} |json_pp &
#        num=`expr $num + 1`
#done

TB_IMAGE_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/image | jq -r '.image[].id'`
#echo $TB_IMAGE_IDS | json_pp

if [ -n "$TB_IMAGE_IDS" ]
then
        #TB_IMAGE_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/image | jq -r '.image[].id'`
        for TB_IMAGE_ID in ${TB_IMAGE_IDS}
        do
                echo ....Get ${TB_IMAGE_ID} ...
                curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/image/${TB_IMAGE_ID} | json_pp
        done
else
        echo ....no images found
fi
