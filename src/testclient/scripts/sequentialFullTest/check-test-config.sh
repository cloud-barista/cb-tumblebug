#!/bin/bash

echo "####################################################################"
echo "## Check test config file (-n deveoperPrefix -f ../testSetCustom.env)"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}

echo ""
echo "[Configuration in ($TestSetFile) & (../conf.env) files]"
echo "1) TumblebugServer : $TumblebugServer // SpiderServer : $SpiderServer"
echo "2) NameSpace ID : $NSID // MCIS ID : $MCISID"

INDEXX=${NumCSP}
echo "3) Enabled CSPs for $MCISID"
for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	CSP=${CSPType[$cspi]}
	INDEXY=${NumRegion[$cspi]}
	echo "[$cspi] $CSP (enabled regions : $INDEXY)"
done
echo ""
for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${NumRegion[$cspi]}
	CSP=${CSPType[$cspi]}
	echo "[$cspi] $CSP details"
	for ((cspj = 1; cspj <= INDEXY; cspj++)); do
		echo "[$cspi,$cspj] ${RegionName[$cspi,$cspj]}" 
		echo "- VM SPEC : ${SPEC_NAME[$cspi,$cspj]}"
		echo "- VM IMAGE : ${IMAGE_NAME[$cspi,$cspj]}"
	done
	echo ""
done


echo ""
