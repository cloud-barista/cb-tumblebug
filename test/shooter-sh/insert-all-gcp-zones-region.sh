#!/bin/bash
RESTSERVER=localhost

LOCS=(`cat gcp-zones-regions-list.txt |grep "UP"  |awk '{print $1}'`)

for ZONE in "${LOCS[@]}"
do
#	echo zone: $ZONE

	LEN=`echo ${#ZONE}`
	NUM=`expr $LEN - 2`
	REGION=`expr substr $ZONE 1 $NUM`

	#echo $LEN, $NUM, $ZONE, $REGION
	echo $REGION, $ZONE

	curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"gcp-'$ZONE'","ProviderName":"GCP", "KeyValueInfoList": [{"Key":"Region", "Value":"'$REGION'"}, {"Key":"Zone", "Value":"'$ZONE'"}]}'
	curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"gcp-'$ZONE'-config","ProviderName":"GCP", "DriverName":"gcp-driver01", "CredentialName":"gcp-credential01", "RegionName":"gcp-'$ZONE'"}'

done
