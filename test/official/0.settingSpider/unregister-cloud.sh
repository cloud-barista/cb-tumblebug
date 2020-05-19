#!/bin/bash
source ../conf.env
source ../credentials.conf

echo "####################################################################"
echo "## 0. Remove All Cloud Connction Config(s)"
echo "####################################################################"

CSP=${1}
POSTFIX=${2:-developer}
if [ "${CSP}" == "aws" ]; then
	echo "[Test for AWS]"
	INDEX=1
elif [ "${CSP}" == "azure" ]; then
	echo "[Test for Azure]"
	INDEX=2
elif [ "${CSP}" == "gcp" ]; then
	echo "[Test for GCP]"
	INDEX=3
else
	echo "[No acceptable argument was provided (aws, azure, gcp, ..). Default: Test for AWS]"
	CSP="aws"
	INDEX=1
fi

RESTSERVER=localhost

# for Cloud Connection Config Info
curl -sX DELETE http://$RESTSERVER:1024/spider/connectionconfig/${CONN_CONFIG[INDEX]}

# for Cloud Region Info
curl -sX DELETE http://$RESTSERVER:1024/spider/region/${RegionName[INDEX]}

# for Cloud Credential Info
curl -sX DELETE http://$RESTSERVER:1024/spider/credential/${CredentialName[INDEX]}
 
# for Cloud Driver Info
curl -sX DELETE http://$RESTSERVER:1024/spider/driver/${DriverName[INDEX]}
