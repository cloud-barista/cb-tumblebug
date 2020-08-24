package cmd

import (
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"

	sp_api "github.com/cloud-barista/cb-spider/interface/api"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/cbadm/proc"
	"github.com/cloud-barista/cb-tumblebug/src/api/grpc/logger"
	tb_api "github.com/cloud-barista/cb-tumblebug/src/api/grpc/request"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// ===== [ Implementations ] =====

// ===== [ Private Functions ] =====

func readInDataFromFile() {
	logger := logger.NewLogger()
	if inData == "" {
		if inFile != "" {
			dat, err := ioutil.ReadFile(inFile)
			if err != nil {
				logger.Error("failed to read file : ", inFile)
				return
			}
			inData = string(dat)
		}
	}
}

// ===== [ Public Functions ] =====

// SetupAndRun - SPIDER GRPC CLI 구동
func SetupAndRun(cmd *cobra.Command, args []string) {
	logger := logger.NewLogger()

	var (
		result string
		err    error

		cim *sp_api.CIMApi

		ns   *tb_api.NSApi   = nil
		mcir *tb_api.MCIRApi = nil
		mcis *tb_api.MCISApi = nil
	)

	// panic 처리
	defer func() {
		if r := recover(); r != nil {
			logger.Error("tbctl is stopped : ", r)
		}
	}()

	if cmd.Parent().Name() == "driver" || cmd.Parent().Name() == "credential" || cmd.Parent().Name() == "region" || cmd.Parent().Name() == "connect-infos" {
		// CIM API 설정
		cim = sp_api.NewCloudInfoManager()
		err = cim.SetConfigPath(configFile)
		if err != nil {
			logger.Error("failed to set config : ", err)
			return
		}
		err = cim.Open()
		if err != nil {
			logger.Error("cim api open failed : ", err)
			return
		}
		defer cim.Close()
	}

	if cmd.Parent().Name() == "namespaces" {
		// NS API 설정
		ns = tb_api.NewNSManager()
		err = ns.SetConfigPath(configFile)
		if err != nil {
			logger.Error("failed to set config : ", err)
			return
		}
		err = ns.Open()
		if err != nil {
			logger.Error("namespace api open failed : ", err)
			return
		}
		defer ns.Close()
	}

	if cmd.Parent().Name() == "images" || cmd.Parent().Name() == "networks" || cmd.Parent().Name() == "securitygroup" || cmd.Parent().Name() == "keypairs" || cmd.Parent().Name() == "specs" {
		// MCIR API 설정
		mcir = tb_api.NewMCIRManager()
		err = mcir.SetConfigPath(configFile)
		if err != nil {
			logger.Error("failed to set config : ", err)
			return
		}
		err = mcir.Open()
		if err != nil {
			logger.Error("namespace api open failed : ", err)
			return
		}
		defer mcir.Close()
	}

	if cmd.Parent().Name() == "mcis" {
		// MCIS API 설정
		mcis = tb_api.NewMCISManager()
		err = mcis.SetConfigPath(configFile)
		if err != nil {
			logger.Error("failed to set config : ", err)
			return
		}
		err = mcis.Open()
		if err != nil {
			logger.Error("mcis api open failed : ", err)
			return
		}
		defer mcis.Close()
	}

	// 입력 파라미터 처리
	if outType != "json" && outType != "yaml" {
		logger.Error("failed to validate --output parameter : ", outType)
		return
	}
	if inType != "json" && inType != "yaml" {
		logger.Error("failed to validate --input parameter : ", inType)
		return
	}

	if cmd.Parent().Name() == "driver" || cmd.Parent().Name() == "credential" || cmd.Parent().Name() == "region" || cmd.Parent().Name() == "connect-infos" {
		cim.SetInType(inType)
		cim.SetOutType(outType)
	}
	if cmd.Parent().Name() == "namespaces" {
		ns.SetInType(inType)
		ns.SetOutType(outType)
	}
	if cmd.Parent().Name() == "images" || cmd.Parent().Name() == "networks" || cmd.Parent().Name() == "securitygroup" || cmd.Parent().Name() == "keypairs" || cmd.Parent().Name() == "specs" {
		mcir.SetInType(inType)
		mcir.SetOutType(outType)
	}
	if cmd.Parent().Name() == "mcis" {
		mcis.SetInType(inType)
		mcis.SetOutType(outType)
	}

	logger.Debug("--input parameter value : ", inType)
	logger.Debug("--output parameter value : ", outType)

	result = ""
	err = nil

	switch cmd.Parent().Name() {
	case "cbadm":
		switch cmd.Name() {
		case "apply":
			fmt.Printf("yaml apply command is not implemented")
		case "get":
			fmt.Printf("yaml get command is not implemented")
		case "list":
			fmt.Printf("yaml list command is not implemented")
		case "remove":
			fmt.Printf("yaml remove command is not implemented")
		}
	case "driver":
		switch cmd.Name() {
		case "create":
			result, err = cim.CreateCloudDriver(inData)
		case "list":
			result, err = cim.ListCloudDriver()
		case "get":
			result, err = cim.GetCloudDriverByParam(driverName)
		case "delete":
			result, err = cim.DeleteCloudDriverByParam(driverName)
		}
	case "credential":
		switch cmd.Name() {
		case "create":
			result, err = cim.CreateCredential(inData)
		case "list":
			result, err = cim.ListCredential()
		case "get":
			result, err = cim.GetCredentialByParam(credentialName)
		case "delete":
			result, err = cim.DeleteCredentialByParam(credentialName)
		}
	case "region":
		switch cmd.Name() {
		case "create":
			result, err = cim.CreateRegion(inData)
		case "list":
			result, err = cim.ListRegion()
		case "get":
			result, err = cim.GetRegionByParam(regionName)
		case "delete":
			result, err = cim.DeleteRegionByParam(regionName)
		}
	case "connect-infos":
		switch cmd.Name() {
		case "create":
			result, err = cim.CreateConnectionConfig(inData)
		case "list":
			result, err = proc.ListConnectInfos(cim)
		case "get":
			result, err = proc.GetConnectInfos(cim, configName)
		case "delete":
			result, err = cim.DeleteConnectionConfigByParam(configName)
		}
	case "namespaces":
		switch cmd.Name() {
		case "create":
			result, err = ns.CreateNS(inData)
		case "list":
			result, err = ns.ListNS()
		case "get":
			result, err = ns.GetNSByParam(nameSpaceID)
		case "delete":
			result, err = ns.DeleteNSByParam(nameSpaceID)
		}
	case "images":
		switch cmd.Name() {
		case "create":
			result, err = mcir.CreateImageWithInfo(inData)
		case "list":
			result, err = mcir.ListImageByParam(nameSpaceID)
		case "get":
			result, err = mcir.GetImageByParam(nameSpaceID, resourceID)
		case "list-csp":
			result, err = mcir.ListLookupImageByParam(connConfigName)
		case "get-csp":
			result, err = mcir.GetLookupImageByParam(connConfigName, imageId)
		case "delete":
			result, err = mcir.DeleteImageByParam(nameSpaceID, resourceID, force)
		case "fetch":
			result, err = mcir.FetchImageByParam(nameSpaceID)
		}
	case "networks":
		switch cmd.Name() {
		case "create":
			result, err = mcir.CreateVNet(inData)
		case "list":
			result, err = mcir.ListVNetByParam(nameSpaceID)
		case "get":
			result, err = mcir.GetVNetByParam(nameSpaceID, resourceID)
		case "delete":
			result, err = mcir.DeleteVNetByParam(nameSpaceID, resourceID, force)
		}
	case "securitygroup":
		switch cmd.Name() {
		case "create":
			result, err = mcir.CreateSecurityGroup(inData)
		case "list":
			result, err = mcir.ListSecurityGroupByParam(nameSpaceID)
		case "get":
			result, err = mcir.GetSecurityGroupByParam(nameSpaceID, resourceID)
		case "delete":
			result, err = mcir.DeleteSecurityGroupByParam(nameSpaceID, resourceID, force)
		}
	case "keypairs":
		switch cmd.Name() {
		case "create":
			result, err = mcir.CreateSshKey(inData)
		case "list":
			result, err = mcir.ListSshKeyByParam(nameSpaceID)
		case "get":
			result, err = mcir.GetSshKeyByParam(nameSpaceID, resourceID)
		case "save":
			result, err = proc.SaveSshKey(mcir, nameSpaceID, resourceID, sshSaveFileName)
		case "delete":
			result, err = mcir.DeleteSshKeyByParam(nameSpaceID, resourceID, force)
		}
	case "specs":
		switch cmd.Name() {
		case "create":
			result, err = mcir.CreateSpecWithInfo(inData)
		case "list":
			result, err = mcir.ListSpecByParam(nameSpaceID)
		case "get":
			result, err = mcir.GetSpecByParam(nameSpaceID, resourceID)
		case "list-csp":
			result, err = mcir.ListLookupSpecByParam(connConfigName)
		case "get-csp":
			result, err = mcir.GetLookupSpecByParam(connConfigName, specName)
		case "delete":
			result, err = mcir.DeleteSpecByParam(nameSpaceID, resourceID, force)
		case "fetch":
			result, err = mcir.FetchSpecByParam(nameSpaceID)
		}
	case "mcis":
		switch cmd.Name() {
		case "create":
			result, err = mcis.CreateMcis(inData)
		case "list":
			result, err = mcis.ListMcisByParam(nameSpaceID, option)
		case "get":
			result, err = mcis.GetMcisInfoByParam(nameSpaceID, mcisID)
		case "delete":
			result, err = mcis.DeleteMcisByParam(nameSpaceID, mcisID)
		case "status":
			result, err = mcis.GetMcisStatusByParam(nameSpaceID, mcisID)
		case "suspend":
			result, err = mcis.ControlMcisByParam(nameSpaceID, mcisID, "suspend")
		case "resume":
			result, err = mcis.ControlMcisByParam(nameSpaceID, mcisID, "resume")
		case "reboot":
			result, err = mcis.ControlMcisByParam(nameSpaceID, mcisID, "reboot")
		case "terminate":
			result, err = mcis.ControlMcisByParam(nameSpaceID, mcisID, "terminate")
		case "add-vm":
			result, err = mcis.CreateMcisVM(inData)
		case "list-vm":
			result, err = proc.ListMcisVM(mcis, nameSpaceID, mcisID)
		case "get-vm":
			result, err = mcis.GetMcisVMInfoByParam(nameSpaceID, mcisID, vmID)
		case "del-vm":
			result, err = mcis.DeleteMcisVMByParam(nameSpaceID, mcisID, vmID)
		case "status-vm":
			result, err = mcis.GetMcisVMStatusByParam(nameSpaceID, mcisID, vmID)
		case "suspend-vm":
			result, err = mcis.ControlMcisVMByParam(nameSpaceID, mcisID, vmID, "suspend")
		case "resume-vm":
			result, err = mcis.ControlMcisVMByParam(nameSpaceID, mcisID, vmID, "resume")
		case "reboot-vm":
			result, err = mcis.ControlMcisVMByParam(nameSpaceID, mcisID, vmID, "reboot")
		case "terminate-vm":
			result, err = mcis.ControlMcisVMByParam(nameSpaceID, mcisID, vmID, "terminate")
		case "command":
			result, err = mcis.CmdMcis(inData)
		case "command-vm":
			result, err = mcis.CmdMcisVm(inData)
		case "deploy-milkyway":
			result, err = mcis.InstallAgentToMcis(inData)
		case "access-vm":
			fmt.Printf("mcis access-vm command is not implemented")
		case "benchmark":
			fmt.Printf("mcis benchmark command is not implemented")
		}
	}

	if err != nil {
		logger.Error("failed to run command: ", err)
	}

	fmt.Printf("%s\n", result)
}
