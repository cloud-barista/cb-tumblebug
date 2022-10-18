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

// SetupAndRun : setup and run Cloud-Barista gRPC CLI
func SetupAndRun(cmd *cobra.Command, args []string) {
	logger := logger.NewLogger()

	var (
		result string
		err    error

		cim *sp_api.CIMApi

		ns     *tb_api.NSApi      = nil
		mcir   *tb_api.MCIRApi    = nil
		mcis   *tb_api.MCISApi    = nil
		tbutil *tb_api.UtilityApi = nil
	)

	// panic handling
	defer func() {
		if r := recover(); r != nil {
			logger.Error("tbctl is stopped : ", r)
		}
	}()

	if cmd.Parent().Name() == "driver" || cmd.Parent().Name() == "credential" || cmd.Parent().Name() == "region" || cmd.Parent().Name() == "connect-info" {
		// CIM API
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

	if cmd.Parent().Name() == "namespace" {
		// NS API
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

	if cmd.Parent().Name() == "image" || cmd.Parent().Name() == "network" || cmd.Parent().Name() == "securitygroup" || cmd.Parent().Name() == "keypair" || cmd.Parent().Name() == "spec" || cmd.Parent().Name() == "commonresource" || cmd.Parent().Name() == "defaultresource" {
		// MCIR API
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
		// MCIS API
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

	if cmd.Parent().Name() == "util" || cmd.Parent().Name() == "config" {
		// Utility API
		tbutil = tb_api.NewUtilityManager()
		err = tbutil.SetConfigPath(configFile)
		if err != nil {
			logger.Error("failed to set config : ", err)
			return
		}
		err = tbutil.Open()
		if err != nil {
			logger.Error("mcis api open failed : ", err)
			return
		}
		defer tbutil.Close()
	}

	// Validate input parameters
	if outType != "json" && outType != "yaml" {
		logger.Error("failed to validate --output parameter : ", outType)
		return
	}
	if inType != "json" && inType != "yaml" {
		logger.Error("failed to validate --input parameter : ", inType)
		return
	}

	if cmd.Parent().Name() == "driver" || cmd.Parent().Name() == "credential" || cmd.Parent().Name() == "region" || cmd.Parent().Name() == "connect-info" {
		cim.SetInType(inType)
		cim.SetOutType(outType)
	}
	if cmd.Parent().Name() == "namespace" {
		ns.SetInType(inType)
		ns.SetOutType(outType)
	}
	if cmd.Parent().Name() == "image" || cmd.Parent().Name() == "network" || cmd.Parent().Name() == "securitygroup" || cmd.Parent().Name() == "keypair" || cmd.Parent().Name() == "spec" || cmd.Parent().Name() == "commonresource" || cmd.Parent().Name() == "defaultresource" {
		mcir.SetInType(inType)
		mcir.SetOutType(outType)
	}
	if cmd.Parent().Name() == "mcis" {
		mcis.SetInType(inType)
		mcis.SetOutType(outType)
	}
	if cmd.Parent().Name() == "util" || cmd.Parent().Name() == "config" {
		tbutil.SetInType(inType)
		tbutil.SetOutType(outType)
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
	case "connect-info":
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
	case "namespace":
		switch cmd.Name() {
		case "create":
			result, err = ns.CreateNS(inData)
		case "list":
			result, err = ns.ListNS()
		case "list-id":
			result, err = ns.ListNSId()
		case "get":
			result, err = ns.GetNSByParam(nameSpaceID)
		case "delete":
			result, err = ns.DeleteNSByParam(nameSpaceID)
		case "delete-all":
			result, err = ns.DeleteAllNS()
		case "check":
			result, err = ns.CheckNSByParam(nameSpaceID)
		}
	case "image":
		switch cmd.Name() {
		case "create":
			result, err = mcir.CreateImageWithInfo(inData)
		case "create-id":
			result, err = mcir.CreateImageWithID(inData)
		case "list":
			result, err = mcir.ListImageByParam(nameSpaceID)
		case "list-id":
			result, err = mcir.ListImageIdByParam(nameSpaceID)
		case "get":
			result, err = mcir.GetImageByParam(nameSpaceID, resourceID)
		case "list-csp":
			result, err = mcir.ListLookupImageByParam(connConfigName)
		case "get-csp":
			result, err = mcir.GetLookupImageByParam(connConfigName, cspImageId)
		case "delete":
			result, err = mcir.DeleteImageByParam(nameSpaceID, resourceID, force)
		case "delete-all":
			result, err = mcir.DeleteAllImageByParam(nameSpaceID, force)
		case "fetch":
			result, err = mcir.FetchImageByParam(connConfigName, nameSpaceID)
		case "search":
			result, err = mcir.SearchImage(inData)
		case "update":
			result, err = mcir.UpdateImage(inData)
		}
	case "network":
		switch cmd.Name() {
		case "create":
			result, err = mcir.CreateVNet(inData)
		case "list":
			result, err = mcir.ListVNetByParam(nameSpaceID)
		case "list-id":
			result, err = mcir.ListVNetIdByParam(nameSpaceID)
		case "get":
			result, err = mcir.GetVNetByParam(nameSpaceID, resourceID)
		case "delete":
			result, err = mcir.DeleteVNetByParam(nameSpaceID, resourceID, force)
		case "delete-all":
			result, err = mcir.DeleteAllVNetByParam(nameSpaceID, force)
		}
	case "securitygroup":
		switch cmd.Name() {
		case "create":
			result, err = mcir.CreateSecurityGroup(inData)
		case "list":
			result, err = mcir.ListSecurityGroupByParam(nameSpaceID)
		case "list-id":
			result, err = mcir.ListSecurityGroupIdByParam(nameSpaceID)
		case "get":
			result, err = mcir.GetSecurityGroupByParam(nameSpaceID, resourceID)
		case "delete":
			result, err = mcir.DeleteSecurityGroupByParam(nameSpaceID, resourceID, force)
		case "delete-all":
			result, err = mcir.DeleteAllSecurityGroupByParam(nameSpaceID, force)
		}
	case "keypair":
		switch cmd.Name() {
		case "create":
			result, err = mcir.CreateSshKey(inData)
		case "list":
			result, err = mcir.ListSshKeyByParam(nameSpaceID)
		case "list-id":
			result, err = mcir.ListSshKeyIdByParam(nameSpaceID)
		case "get":
			result, err = mcir.GetSshKeyByParam(nameSpaceID, resourceID)
		case "save":
			result, err = proc.SaveSshKey(mcir, nameSpaceID, resourceID, sshSaveFileName)
		case "delete":
			result, err = mcir.DeleteSshKeyByParam(nameSpaceID, resourceID, force)
		case "delete-all":
			result, err = mcir.DeleteAllSshKeyByParam(nameSpaceID, force)
		}
	case "spec":
		switch cmd.Name() {
		case "create":
			result, err = mcir.CreateSpecWithInfo(inData)
		case "create-id":
			result, err = mcir.CreateSpecWithSpecName(inData)
		case "list":
			result, err = mcir.ListSpecByParam(nameSpaceID)
		case "list-id":
			result, err = mcir.ListSpecIdByParam(nameSpaceID)
		case "get":
			result, err = mcir.GetSpecByParam(nameSpaceID, resourceID)
		case "list-csp":
			result, err = mcir.ListLookupSpecByParam(connConfigName)
		case "get-csp":
			result, err = mcir.GetLookupSpecByParam(connConfigName, cspSpecName)
		case "delete":
			result, err = mcir.DeleteSpecByParam(nameSpaceID, resourceID, force)
		case "delete-all":
			result, err = mcir.DeleteAllSpecByParam(nameSpaceID, force)
		case "fetch":
			result, err = mcir.FetchSpecByParam(connConfigName, nameSpaceID)
		case "filter":
			result, err = mcir.FilterSpec(inData)
		case "filter-by-range":
			result, err = mcir.FilterSpecsByRange(inData)
		case "sort":
			result, err = mcir.SortSpecs(inData)
		case "update":
			result, err = mcir.UpdateSpec(inData)
		}
	case "commonresource":
		switch cmd.Name() {
		case "load":
			result, err = mcir.LoadCommonResource()
		}
	case "defaultresource":
		switch cmd.Name() {
		case "load":
			result, err = mcir.LoadDefaultResourceByParam(nameSpaceID, resourceType, connConfigName)
			// case "delete":
			// 	result, err = mcir.DeleteDefaultResource()
		}
	case "mcis":
		switch cmd.Name() {
		case "create":
			result, err = mcis.CreateMcis(inData)
		case "list":
			result, err = mcis.ListMcisByParam(nameSpaceID)
		case "list-id":
			result, err = mcis.ListMcisIdByParam(nameSpaceID)
		case "get":
			result, err = mcis.GetMcisInfoByParam(nameSpaceID, mcisID)
		case "delete":
			result, err = mcis.DeleteMcisByParam(nameSpaceID, mcisID, option)
		case "delete-all":
			result, err = mcis.DeleteAllMcisByParam(nameSpaceID)
		case "status-list":
			result, err = mcis.ListMcisStatusByParam(nameSpaceID)
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
		case "group-vm":
			result, err = mcis.CreateMcisSubGroup(inData)
		case "list-vm":
			result, err = proc.ListMcisVM(mcis, nameSpaceID, mcisID)
		case "list-vm-id":
			result, err = mcis.ListMcisVmIdByParam(nameSpaceID, mcisID)
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
			result, err = mcis.InstallBenchmarkAgentToMcis(inData)
		case "access-vm":
			fmt.Printf("mcis access-vm command is not implemented")
		case "benchmark":
			if action == "all" {
				result, err = mcis.GetAllBenchmarkByParam(nameSpaceID, mcisID, host)
			} else {
				result, err = mcis.GetBenchmarkByParam(nameSpaceID, mcisID, action, host)
			}
		case "install-mon":
			result, err = mcis.InstallMonitorAgentToMcis(inData)
		case "get-mon":
			result, err = mcis.GetMonitorDataByParam(nameSpaceID, mcisID, metric)
		case "create-policy":
			result, err = mcis.CreateMcisPolicy(inData)
		case "list-policy":
			result, err = mcis.ListMcisPolicyByParam(nameSpaceID)
		case "get-policy":
			result, err = mcis.GetMcisPolicyByParam(nameSpaceID, mcisID)
		case "delete-policy":
			result, err = mcis.DeleteMcisPolicyByParam(nameSpaceID, mcisID)
		case "delete-all-policy":
			result, err = mcis.DeleteAllMcisPolicyByParam(nameSpaceID)
		case "recommend":
			result, err = mcis.RecommendMcis(inData)
		case "recommend-vm":
			result, err = mcis.RecommendVM(inData)
		}
	case "util":
		switch cmd.Name() {
		case "list-cc":
			result, err = tbutil.ListConnConfig()
		case "get-cc":
			result, err = tbutil.GetConnConfigByParam(connConfigName)
		case "list-region":
			result, err = tbutil.ListRegion()
		case "get-region":
			result, err = tbutil.GetRegionByParam(regionName)
		case "inspect-mcir":
			result, err = tbutil.InspectMcirResourcesByParam(connConfigName, resourceType)
		case "inspect-vm":
			result, err = tbutil.InspectVmResourcesByParam(connConfigName)
		case "list-obj":
			result, err = tbutil.ListObjectByParam(objKey)
		case "get-obj":
			result, err = tbutil.GetObjectByParam(objKey)
		case "delete-obj":
			result, err = tbutil.DeleteObjectByParam(objKey)
		case "delete-all-obj":
			result, err = tbutil.DeleteAllObjectByParam(objKey)
		}
	case "config":
		switch cmd.Name() {
		case "create":
			result, err = tbutil.CreateConfig(inData)
		case "list":
			result, err = tbutil.ListConfig()
		case "get":
			result, err = tbutil.GetConfigByParam(configId)
		case "init":
			result, err = tbutil.InitConfigByParam(configId)
		case "init-all":
			result, err = tbutil.InitAllConfig()
		}
	}

	if err != nil {
		if outType == "yaml" {
			fmt.Fprintf(cmd.OutOrStdout(), "message: %v\n", err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "{\"message\": \"%v\"}\n", err)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", result)
	}
}
