#!/bin/bash

source ../credentials.conf

echo "####################################################################"
echo "## 0. List Cloud Connction Config(s)"
echo "####################################################################"

#INDEX=${1}

RESTSERVER=localhost

# for Cloud Connection Config Info
curl -sX GET http://$RESTSERVER:1024/spider/connectionconfig | json_pp


# for Cloud Region Info
curl -sX GET http://$RESTSERVER:1024/spider/region | json_pp


# for Cloud Credential Info
curl -sX GET http://$RESTSERVER:1024/spider/credential | json_pp

 
# for Cloud Driver Info
curl -sX GET http://$RESTSERVER:1024/spider/driver | json_pp