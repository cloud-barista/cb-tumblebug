#!/bin/bash

echo "####################################################################"
echo "## Check test config file (-n deveoperPrefix -f ../testSetCustom.env -x numOfVMsInEachVMGroup)"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}

echo ""
echo "[Configuration in ($TestSetFile) & (../conf.env) files]"
echo "1) Tumblebug Server : $TumblebugServer // Spider Server : $SpiderServer"


INDEXX=${NumCSP}
echo "2) Enabled CSPs and regions in $TestSetFile"
for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	CSP=${CSPType[$cspi]}
	INDEXY=${NumRegion[$cspi]}
	echo " - [$cspi] $CSP (enabled regions : $INDEXY)"
done
echo ""

for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${NumRegion[$cspi]}
	CSP=${CSPType[$cspi]}
	echo "   [$cspi] $CSP details"
	for ((cspj = 1; cspj <= INDEXY; cspj++)); do
		echo "   [$cspi,$cspj] ${RegionName[$cspi,$cspj]}" 
		echo "    - VM SPEC : ${SPEC_NAME[$cspi,$cspj]}"
		echo "    - VM IMAGE : ${IMAGE_NAME[$cspi,$cspj]}"
	done
	echo ""
done

echo "3) MCIS Configuration"
echo " - NameSpace ID : $NSID"
echo " - MCIS ID : $MCISID"
echo " - Number of VMs"

for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${NumRegion[$cspi]}
	CSP=${CSPType[$cspi]}
	TOTALVM=$((1 * INDEXY * NUMVM))
	echo "   - [$cspi] VMs($TOTALVM) = $CSP(1) * Region($INDEXY) * VMgroup($NUMVM)"
done

echo ""

