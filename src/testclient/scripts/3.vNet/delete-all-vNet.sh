#!/bin/bash

function CallTB() {
	echo "- Delete all vNets"

	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/resources/vNet | jq '.'
}

#function delete_vNet() {
	
	echo "####################################################################"
	echo "## 3. vNet: Delete"
	echo "####################################################################"

	source ../init.sh

	CallTB

#}

#delete_vNet
