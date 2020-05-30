#!/bin/bash
source ../conf.env
source ../credentials.conf

echo "####################################################################"
echo "## 0. Get Cloud Connction Config"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}
if [ "${CSP}" == "aws" ]; then
	echo "[Test for AWS]"
	INDEX=1
elif [ "${CSP}" == "azure" ]; then
	echo "[Test for Azure]"
	INDEX=2
elif [ "${CSP}" == "gcp" ]; then
	echo "[Test for GCP]"
	INDEX=3
elif [ "${CSP}" == "alibaba" ]; then
	echo "[Test for Alibaba]"
	INDEX=4
else
	echo "[No acceptable argument was provided (aws, azure, gcp, alibaba, ...). Default: Test for AWS]"
	CSP="aws"
	INDEX=1
fi

RESTSERVER=localhost

# for Cloud Connection Config Info
curl -sX GET http://$SpiderServer/spider/connectionconfig/${CONN_CONFIG[$INDEX,$REGION]} | json_pp


# for Cloud Region Info
curl -sX GET http://$SpiderServer/spider/region/${RegionName[$INDEX,$REGION]} | json_pp


# for Cloud Credential Info
curl -sX GET http://$SpiderServer/spider/credential/${CredentialName[INDEX]} | json_pp

 
# for Cloud Driver Info
curl -sX GET http://$SpiderServer/spider/driver/${DriverName[INDEX]} | json_pp

