#!/bin/bash

#function status_infra() {

echo "####################################################################"
echo "## 8. VM: Status Infra"
echo "####################################################################"

source ../init.sh

# if [ "${INDEX}" == "0" ]; then
# 	InfraID=${POSTFIX}
# else
# 	InfraID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
# fi

echo "${InfraID}"

GetInfraOption=status
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/infra/${InfraID}?option=${GetInfraOption} | jq '.'

echo -e "${BOLD}"
echo -e "Table: All VMs in the Infra : ${InfraID}"

echo -e "${NC} ${BLUE} ${BOLD}"
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/infra/${InfraID}?option=${GetInfraOption} |
    jq '.status | .vm | sort_by(.id)' |
    jq -r '(["VM-ID","Status","PublicIP","PrivateIP","CloudType","CloudRegion","CreatedTime"] | (., map(length*"-"))), (.[] | [.id, .status, .publicIp, .privateIp, .location.cloudType, .location.nativeRegion, .createdTime]) | @tsv' |
    column -t
echo -e "${NC}"

#HTTP_CODE=$(curl -o /dev/null -w "%{http_code}\n" -H "${AUTH}" "http://${TumblebugServer}/tumblebug/ns/$NSID/infra/${InfraID}?option=status" --silent)
#echo "Status: $HTTP_CODE"

#}

#status_infra
