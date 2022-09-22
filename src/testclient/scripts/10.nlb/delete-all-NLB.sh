#!/bin/bash

function CallTB() {
	echo "- Delete all NLBs"

	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/nlb | jq ''
}

#function delete_NLB() {
	
	echo "####################################################################"
	echo "## 10. NLB: Delete"
	echo "####################################################################"

	source ../init.sh

	CallTB

#}

#delete_NLB
