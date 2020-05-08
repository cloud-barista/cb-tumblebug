#!/bin/bash

source ../credentials.env

echo "####################################################################"
echo "## 0. Get Cloud Connction Config"
echo "####################################################################"

INDEX=${1}

RESTSERVER=localhost

# for Cloud Connection Config Info
curl -sX GET http://$RESTSERVER:1024/spider/connectionconfig/${CONN_CONFIG[INDEX]} | json_pp


# for Cloud Region Info
curl -sX GET http://$RESTSERVER:1024/spider/region/${RegionName[INDEX]} | json_pp


# for Cloud Credential Info
curl -sX GET http://$RESTSERVER:1024/spider/credential/${CredentialName[INDEX]} | json_pp

 
# for Cloud Driver Info
curl -sX GET http://$RESTSERVER:1024/spider/driver/${DriverName[INDEX]} | json_pp

