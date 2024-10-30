#!/bin/bash

#function status_mci() {

echo "####################################################################"
echo "## 8. VM: Status MCI"
echo "####################################################################"

source ../init.sh

# if [ "${INDEX}" == "0" ]; then
# 	MCIID=${POSTFIX}
# else
# 	MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
# fi

echo "${MCIID}"

GetMCIOption=status
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci/${MCIID}?option=${GetMCIOption} | jq '.'

echo -e "${BOLD}"
echo -e "Table: All VMs in the MCI : ${MCIID}"

echo -e "${NC} ${BLUE} ${BOLD}"
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci/${MCIID}?option=${GetMCIOption} |
    jq '.status | .vm | sort_by(.id)' |
    jq -r '(["VM-ID","Status","PublicIP","PrivateIP","CloudType","CloudRegion","CreatedTime"] | (., map(length*"-"))), (.[] | [.id, .status, .publicIp, .privateIp, .location.cloudType, .location.nativeRegion, .createdTime]) | @tsv' |
    column -t
echo -e "${NC}"

#HTTP_CODE=$(curl -o /dev/null -w "%{http_code}\n" -H "${AUTH}" "http://${TumblebugServer}/tumblebug/ns/$NSID/mci/${MCIID}?option=status" --silent)
#echo "Status: $HTTP_CODE"

#}

#status_mci
