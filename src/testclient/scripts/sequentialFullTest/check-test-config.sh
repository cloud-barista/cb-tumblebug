#!/bin/bash

echo "####################################################################"
echo "## Check test config file (-n deveoperPrefix -f ../testSetCustom.env -x numOfVMsInEachVMGroup)"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}
#echo -e "${NC} ${GREEN} ${BOLD}"
echo ""
echo -e "[${BOLD}Configuration${NC} in ${GREEN} $TestSetFile${NC} & ${GREEN} ../conf.env ${NC} files]"
echo ""
echo -e "${BOLD}1) System Endpoints${NC}"
echo -e " - Tumblebug Server : ${GREEN} $TumblebugServer ${NC}"
echo -e " - Spider Server : ${GREEN} $SpiderServer ${NC}"
echo ""

INDEXX=${NumCSP}
echo -e "${BOLD}2) Enabled CSPs and regions${NC}"

for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${NumRegion[$cspi]}
	CSP=${CSPType[$cspi]}
	echo -e "${GREEN} - [$cspi] $CSP (enabled regions : $INDEXY)${NC}"
	for ((cspj = 1; cspj <= INDEXY; cspj++)); do
		echo -e "${YELLOW}   [$cspi,$cspj] ${RegionName[$cspi,$cspj]}" 
		echo -e "    - VM SPEC : ${SPEC_NAME[$cspi,$cspj]}"
		echo -e "    - VM IMAGE : ${IMAGE_NAME[$cspi,$cspj]} ${NC}"
	done
	echo ""
done

# Count total VMs
TotalVM=0
for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${NumRegion[$cspi]}
	CSP=${CSPType[$cspi]}
	TotalVM=$(($TotalVM + $((1 * INDEXY * NUMVM))))
done

echo -e "${BOLD}3) MCIS Configuration${NC}"
echo -e " - NameSpace ID :${GREEN} $NSID${NC}"
echo -e " - MCIS ID :${GREEN} $MCISID${NC}"
echo -e " - Number of Total VMs :${GREEN} $TotalVM${NC}"

for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${NumRegion[$cspi]}
	CSP=${CSPType[$cspi]}
	RegionVM=$((1 * INDEXY * NUMVM))
	echo -e "   - ${YELLOW}[$cspi] VMs($RegionVM) = $CSP(1) * Region($INDEXY) * Number Of VMs($NUMVM)${NC}"
done

echo ""

