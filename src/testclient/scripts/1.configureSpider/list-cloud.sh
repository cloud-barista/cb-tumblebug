#!/bin/bash

#function list_cloud() {
source ../conf.env
source ../common-functions.sh

echo "####################################################################"
echo "## 0. List Cloud Connction Config(s)"
echo "####################################################################"


# for Cloud Region Info
echo -e "${BLUE}${BOLD}[Cloud Region]${NC}"
echo -e "${BLUE}"
curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/region |
    jq '.region' |
    jq -r '(["RegionNativeName","ProviderName","Region","Zone"] | (., map(length*"-"))), (.[] | [.RegionNativeName, .ProviderName, .KeyValueInfoList[0].Value, .KeyValueInfoList[1].Value]) | @tsv' |
    column -t
echo -e "${NC}"
echo ""

# for Cloud Credential Info
echo "[Cloud Credential]"
curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/credential |
    jq '.credential' |
    jq -r '(["CredentialName","ProviderName"] | (., map(length*"-"))), (.[] | [.CredentialName, .ProviderName]) | @tsv' |
    column -t
echo ""
echo ""

# for Cloud Driver Info
echo "[Cloud Driver]"
curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/driver |
    jq '.driver' |
    jq -r '(["DriverName","ProviderName","DriverLibFileName"] | (., map(length*"-"))), (.[] | [.DriverName, .ProviderName, .DriverLibFileName]) | @tsv' |
    column -t
echo ""
echo ""

# for Cloud Connection Config Info
echo -e "${BLUE}${BOLD}[Cloud Connection Config]${NC}"
echo -e "${BLUE}"
curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/connectionconfig |
    jq '.connectionconfig' |
    jq -r '(["ConfigName","RegionNativeName","CredentialName","DriverName","ProviderName"] | (., map(length*"-"))), (.[] | [.ConfigName, .RegionNativeName, .CredentialName, .DriverName, .ProviderName]) | @tsv' |
    column -t
echo -e "${NC}"
#}

#list_cloud
