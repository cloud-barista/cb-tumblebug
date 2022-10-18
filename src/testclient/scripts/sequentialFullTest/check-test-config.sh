#!/bin/bash

echo "####################################################################"
echo "## Check test config file (-n deveoperPrefix -f ../testSetCustom.env -x numOfVMsInEachSubGroup)"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}
#echo -e "${NC} ${GREEN} ${BOLD}"
echo ""
echo -e "[${BOLD}Configuration${NC} in ${GREEN}${BOLD} $TestSetFile${NC} & ${GREEN}${BOLD} ../conf.env ${NC} files]"
echo ""
echo -e "${BOLD}1) System Endpoints${NC}"
echo -e " - Tumblebug Server : ${GREEN}${BOLD} $TumblebugServer ${NC}"
echo -e " - Spider Server : ${GREEN}${BOLD} $SpiderServer ${NC}"
echo ""

INDEXX=${NumCSP}
echo -e "${BOLD}2) Enabled Clouds and Regions${NC}"

for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${NumRegion[$cspi]}
	CSP=${CSPType[$cspi]}
	echo -e "${GREEN}${BOLD} - [$cspi] Cloud : $CSP (enabled regions : $INDEXY)${NC}"
	for ((cspj = 1; cspj <= INDEXY; cspj++)); do

		if [[ -z "${DISK_TYPE[$cspi,$cspj]}" ]]; then
			RootDiskType="default"
		else
			RootDiskType="${DISK_TYPE[$cspi,$cspj]}"
		fi

		if [[ -z "${DISK_SIZE[$cspi,$cspj]}" ]]; then
			RootDiskSize="default"
		else
			RootDiskSize="${DISK_SIZE[$cspi,$cspj]}"
		fi

		echo -e "${BLUE}${BOLD}   [$cspi,$cspj] Region : ${RegionName[$cspi,$cspj]} (${RegionLocation[$cspi,$cspj]}) ${NC}" 
		echo -e "    - VM SPEC : ${SPEC_NAME[$cspi,$cspj]} "
		echo -e "    - VM DISK : ${RootDiskType} (${RootDiskSize} GB) "
		echo -e "    - VM IMAGE : ${IMAGE_TYPE[$cspi,$cspj]} (${IMAGE_NAME[$cspi,$cspj]}) "
		
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
echo -e " - NameSpace ID :${GREEN}${BOLD} $NSID${NC}"
echo -e " - MCIS ID :${GREEN}${BOLD} $MCISID${NC}"
echo -e " - Number of Total VMs :${GREEN}${BOLD} $TotalVM${NC}"

for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${NumRegion[$cspi]}
	CSP=${CSPType[$cspi]}
	RegionVM=$((1 * INDEXY * NUMVM))
	echo -e "   - ${BLUE}${BOLD}[$cspi] VMs($RegionVM) = $CSP($INDEXY) * Replica($NUMVM)${NC}"
done

echo ""

