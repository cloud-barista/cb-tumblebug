package mcis

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/cloud-barista/cb-tumblebug/src/mcis"
	"github.com/labstack/echo/v4"
)

// MCIS API Proxy

func RestPostMcis(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcis.TbMcisInfo{}
	if err := c.Bind(req); err != nil {
		return err
	}

	key := mcis.CreateMcis(nsId, req)
	mcisId := req.Id

	keyValue, _ := common.CBStore.Get(key)

	/*
		var content struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			//Vm_num         string   `json:"vm_num"`
			Status         string   `json:"status"`
			TargetStatus   string   `json:"targetStatus"`
			TargetAction   string   `json:"targetAction"`
			Vm             []TbVmInfo `json:"vm"`
			Placement_algo string   `json:"placement_algo"`
			Description    string   `json:"description"`
		}
	*/
	content := mcis.TbMcisInfo{}

	json.Unmarshal([]byte(keyValue.Value), &content)

	vmList, err := mcis.ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	for _, v := range vmList {
		vmKey := common.GenMcisKey(nsId, mcisId, v)
		//fmt.Println(vmKey)
		vmKeyValue, _ := common.CBStore.Get(vmKey)
		if vmKeyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		//fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
		vmTmp := mcis.TbVmInfo{}
		json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
		vmTmp.Id = v
		content.Vm = append(content.Vm, vmTmp)
	}

	//mcisStatus, err := GetMcisStatus(nsId, mcisId)
	//content.Status = mcisStatus.Status

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusCreated, content)
}

func RestGetMcis(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	action := c.QueryParam("action")
	fmt.Println("[Get MCIS requested action: " + action)
	if action == "suspend" {
		fmt.Println("[suspend MCIS]")

		err := mcis.ControlMcisAsync(nsId, mcisId, mcis.ActionSuspend)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		mapA := map[string]string{"message": "Suspending the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "resume" {
		fmt.Println("[resume MCIS]")

		err := mcis.ControlMcisAsync(nsId, mcisId, mcis.ActionResume)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		mapA := map[string]string{"message": "Resuming the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "reboot" {
		fmt.Println("[reboot MCIS]")

		err := mcis.ControlMcisAsync(nsId, mcisId, mcis.ActionReboot)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		mapA := map[string]string{"message": "Rebooting the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "terminate" {
		fmt.Println("[terminate MCIS]")

		vmList, err := mcis.ListVmId(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		//fmt.Println("len(vmList) %d ", len(vmList))
		if len(vmList) == 0 {
			mapA := map[string]string{"message": "No VM to terminate in the MCIS"}
			return c.JSON(http.StatusOK, &mapA)
		}

		/*
			for _, v := range vmList {
				ControlVm(nsId, mcisId, v, ActionTerminate)
			}
		*/
		err = mcis.ControlMcisAsync(nsId, mcisId, mcis.ActionTerminate)
		if err != nil {
			mapA := map[string]string{"message": err.Error()}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		mapA := map[string]string{"message": "Terminating the MCIS"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "status" {

		fmt.Println("[status MCIS]")

		vmList, err := mcis.ListVmId(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		if len(vmList) == 0 {
			mapA := map[string]string{"message": "No VM to check in the MCIS"}
			return c.JSON(http.StatusOK, &mapA)
		}
		mcisStatusResponse, err := mcis.GetMcisStatus(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		//fmt.Printf("%+v\n", mcisStatusResponse)
		common.PrintJsonPretty(mcisStatusResponse)

		return c.JSON(http.StatusOK, &mcisStatusResponse)

	} else {

		var content struct {
			Id   string `json:"id"`
			Name string `json:"name"`
			//Vm_num         string   `json:"vm_num"`
			Status         string          `json:"status"`
			TargetStatus   string          `json:"targetStatus"`
			TargetAction   string          `json:"targetAction"`
			Vm             []mcis.TbVmInfo `json:"vm"`
			Placement_algo string          `json:"placement_algo"`
			Description    string          `json:"description"`
		}

		fmt.Println("[Get MCIS for id]" + mcisId)
		key := common.GenMcisKey(nsId, mcisId, "")
		//fmt.Println(key)

		keyValue, _ := common.CBStore.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		//fmt.Println("===============================================")

		json.Unmarshal([]byte(keyValue.Value), &content)

		mcisStatus, err := mcis.GetMcisStatus(nsId, mcisId)
		content.Status = mcisStatus.Status

		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		vmList, err := mcis.ListVmId(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		for _, v := range vmList {
			vmKey := common.GenMcisKey(nsId, mcisId, v)
			//fmt.Println(vmKey)
			vmKeyValue, _ := common.CBStore.Get(vmKey)
			if vmKeyValue == nil {
				mapA := map[string]string{"message": "Cannot find " + key}
				return c.JSON(http.StatusOK, &mapA)
			}
			//fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
			vmTmp := mcis.TbVmInfo{}
			json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
			vmTmp.Id = v

			//get current vm status
			vmStatusInfoTmp, err := mcis.GetVmStatus(nsId, mcisId, v)
			if err != nil {
				common.CBLog.Error(err)
			}
			vmTmp.Status = vmStatusInfoTmp.Status
			vmTmp.TargetStatus = vmStatusInfoTmp.TargetStatus
			vmTmp.TargetAction = vmStatusInfoTmp.TargetAction

			content.Vm = append(content.Vm, vmTmp)
		}
		//fmt.Printf("%+v\n", content)
		common.PrintJsonPretty(content)
		//return by string
		//return c.String(http.StatusOK, keyValue.Value)
		return c.JSON(http.StatusOK, &content)

	}
}

func RestGetAllMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	option := c.QueryParam("option")
	fmt.Println("[Get MCIS List requested with option: " + option)

	var content struct {
		//Name string     `json:"name"`
		Mcis []mcis.TbMcisInfo `json:"mcis"`
	}

	mcisList := mcis.ListMcisId(nsId)

	for _, v := range mcisList {

		key := common.GenMcisKey(nsId, v, "")
		//fmt.Println(key)
		keyValue, _ := common.CBStore.Get(key)
		if keyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
		mcisTmp := mcis.TbMcisInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
		mcisId := v
		mcisTmp.Id = mcisId

		if option == "status" {
			//get current mcis status
			mcisStatus, err := mcis.GetMcisStatus(nsId, mcisId)
			if err != nil {
				common.CBLog.Error(err)
				return err
			}
			mcisTmp.Status = mcisStatus.Status
		} else {
			//Set current mcis status with NullStr
			mcisTmp.Status = ""
		}

		vmList, err := mcis.ListVmId(nsId, mcisId)
		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		for _, v1 := range vmList {
			vmKey := common.GenMcisKey(nsId, mcisId, v1)
			//fmt.Println(vmKey)
			vmKeyValue, _ := common.CBStore.Get(vmKey)
			if vmKeyValue == nil {
				mapA := map[string]string{"message": "Cannot find " + key}
				return c.JSON(http.StatusOK, &mapA)
			}
			//fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
			//vmTmp := vmOverview{}
			vmTmp := mcis.TbVmInfo{}
			json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
			vmTmp.Id = v1

			if option == "status" {
				//get current vm status
				vmStatusInfoTmp, err := mcis.GetVmStatus(nsId, mcisId, v1)
				if err != nil {
					common.CBLog.Error(err)
				}
				vmTmp.Status = vmStatusInfoTmp.Status
			} else {
				//Set current vm status with NullStr
				vmTmp.Status = ""
			}

			mcisTmp.Vm = append(mcisTmp.Vm, vmTmp)
		}

		content.Mcis = append(content.Mcis, mcisTmp)

	}
	//fmt.Printf("content %+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusOK, &content)

}

func RestPutMcis(c echo.Context) error {
	return nil
}

func RestDelMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	err := mcis.DelMcis(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the MCIS"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "Deleting the MCIS info"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestDelAllMcis(c echo.Context) error {
	nsId := c.Param("nsId")

	mcisList := mcis.ListMcisId(nsId)

	if len(mcisList) == 0 {
		mapA := map[string]string{"message": "No MCIS to delete"}
		return c.JSON(http.StatusOK, &mapA)
	}

	for _, v := range mcisList {
		err := mcis.DelMcis(nsId, v)
		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{"message": "Failed to delete All MCISs"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}
	}

	mapA := map[string]string{"message": "All MCISs has been deleted"}
	return c.JSON(http.StatusOK, &mapA)

}

func RestPostMcisRecommand(c echo.Context) error {

	nsId := c.Param("nsId")

	req := &mcis.McisRecommendReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	var content struct {
		//Vm_req          []TbVmRecommendReq    `json:"vm_req"`
		Vm_recommend    []mcis.TbVmRecommendInfo `json:"vm_recommend"`
		Placement_algo  string                   `json:"placement_algo"`
		Placement_param []common.KeyValue        `json:"placement_param"`
	}
	//content.Vm_req = req.Vm_req
	content.Placement_algo = req.Placement_algo
	content.Placement_param = req.Placement_param

	vmList := req.Vm_req

	for i, v := range vmList {
		vmTmp := mcis.TbVmRecommendInfo{}
		//vmTmp.Request_name = v.Request_name
		vmTmp.Vm_req = req.Vm_req[i]
		vmTmp.Placement_algo = v.Placement_algo
		vmTmp.Placement_param = v.Placement_param

		var err error
		vmTmp.Vm_priority, err = mcis.GetRecommendList(nsId, v.Vcpu_size, v.Memory_size, v.Disk_size)

		if err != nil {
			common.CBLog.Error(err)
			mapA := map[string]string{"message": "Failed to recommend MCIS"}
			return c.JSON(http.StatusFailedDependency, &mapA)
		}

		content.Vm_recommend = append(content.Vm_recommend, vmTmp)
	}
	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusCreated, content)
}

func RestPostCmdMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	vmIp := mcis.GetVmIp(nsId, mcisId, vmId)

	//fmt.Printf("[vmIp] " +vmIp)

	//sshKey := req.Ssh_key
	cmd := req.Command

	// find vaild username
	userName, sshKey := mcis.GetVmSshKey(nsId, mcisId, vmId)
	userNames := []string{
		mcis.SshDefaultUserName01,
		mcis.SshDefaultUserName02,
		mcis.SshDefaultUserName03,
		mcis.SshDefaultUserName04,
		userName,
		req.User_name,
	}
	userName = mcis.VerifySshUserName(vmIp, userNames, sshKey)
	if userName == "" {
		return c.JSON(http.StatusInternalServerError, errors.New("No vaild username"))
	}

	//fmt.Printf("[userName] " +userName)

	fmt.Println("[SSH] " + mcisId + "/" + vmId + "(" + vmIp + ")" + "with userName:" + userName)
	fmt.Println("[CMD] " + cmd)

	if result, err := mcis.RunSSH(vmIp, userName, sshKey, cmd); err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	} else {
		response := echo.Map{}
		response["result"] = *result
		return c.JSON(http.StatusOK, response)
	}
}

func RestPostCmdMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	type contentSub struct {
		Mcis_id string `json:"mcis_id"`
		Vm_id   string `json:"vm_id"`
		Vm_ip   string `json:"vm_ip"`
		Result  string `json:"result"`
	}
	var content struct {
		Result_array []contentSub `json:"result_array"`
	}

	vmList, err := mcis.ListVmId(nsId, mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	//goroutine sync wg
	var wg sync.WaitGroup

	var resultArray []mcis.SshCmdResult

	for _, v := range vmList {
		wg.Add(1)

		vmId := v
		vmIp := mcis.GetVmIp(nsId, mcisId, vmId)

		cmd := req.Command

		// userName, sshKey := GetVmSshKey(nsId, mcisId, vmId)
		// if (userName == "") {
		// 	userName = req.User_name
		// }
		// if (userName == "") {
		// 	userName = sshDefaultUserName
		// }
		// find vaild username
		userName, sshKey := mcis.GetVmSshKey(nsId, mcisId, vmId)
		userNames := []string{
			mcis.SshDefaultUserName01,
			mcis.SshDefaultUserName02,
			mcis.SshDefaultUserName03,
			mcis.SshDefaultUserName04,
			userName,
			req.User_name,
		}
		userName = mcis.VerifySshUserName(vmIp, userNames, sshKey)

		fmt.Println("[SSH] " + mcisId + "/" + vmId + "(" + vmIp + ")" + "with userName:" + userName)
		fmt.Println("[CMD] " + cmd)

		go mcis.RunSSHAsync(&wg, vmId, vmIp, userName, sshKey, cmd, &resultArray)

	}
	wg.Wait() //goroutine sync wg

	for _, v := range resultArray {

		resultTmp := contentSub{}
		resultTmp.Mcis_id = mcisId
		resultTmp.Vm_id = v.Vm_id
		resultTmp.Vm_ip = v.Vm_ip
		resultTmp.Result = v.Result
		content.Result_array = append(content.Result_array, resultTmp)
		//fmt.Println("result from goroutin " + v)
	}

	//fmt.Printf("%+v\n", content)
	common.PrintJsonPretty(content)

	return c.JSON(http.StatusOK, content)

}

func RestPostInstallAgentToMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	req := &mcis.McisCmdReq{}
	if err := c.Bind(req); err != nil {
		return err
	}

	content, err := mcis.InstallAgentToMcis(nsId, mcisId, req)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return c.JSON(http.StatusOK, content)
}

// VM API Proxy

func RestPostMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	/*
		req := &mcis.TbVmReq{}
		if err := c.Bind(req); err != nil {
			return err
		}

		vmInfoData := mcis.TbVmInfo{}
		//vmInfoData.Id = common.GenUuid()
		vmInfoData.Id = common.GenId(req.Name)
		//req.Id = vmInfoData.Id
		//vmInfoData.CspVmName = req.CspVmName // will be deprecated

		//vmInfoData.Placement_algo = req.Placement_algo

		//vmInfoData.Location = req.Location
		//vmInfoData.Cloud_id = req.
		vmInfoData.Description = req.Description

		//vmInfoData.CspSpecId = req.CspSpecId

		//vmInfoData.Vcpu_size = req.Vcpu_size
		//vmInfoData.Memory_size = req.Memory_size
		//vmInfoData.Disk_size = req.Disk_size
		//vmInfoData.Disk_type = req.Disk_type

		//vmInfoData.CspImageName = req.CspImageName

		//vmInfoData.CspSecurityGroupIds = req.CspSecurityGroupIds
		//vmInfoData.CspVirtualNetworkId = "TBD"
		//vmInfoData.Subnet = "TBD"
		//vmInfoData.CspImageName = "TBD"
		//vmInfoData.CspSpecId = "TBD"

		//vmInfoData.PublicIP = "Not assigned yet"
		//vmInfoData.CspVmId = "Not assigned yet"
		//vmInfoData.PublicDNS = "Not assigned yet"
		vmInfoData.Status = "Creating"

		vmInfoData.Name = req.Name
		vmInfoData.ConnectionName = req.ConnectionName
		vmInfoData.SpecId = req.SpecId
		vmInfoData.ImageId = req.ImageId
		vmInfoData.VNetId = req.VNetId
		vmInfoData.SubnetId = req.SubnetId
		//vmInfoData.Vnic_id = req.Vnic_id
		//vmInfoData.Public_ip_id = req.Public_ip_id
		vmInfoData.SecurityGroupIds = req.SecurityGroupIds
		vmInfoData.SshKeyId = req.SshKeyId
		vmInfoData.Description = req.Description

		vmInfoData.ConnectionName = req.ConnectionName
	*/

	vmInfoData := mcis.TbVmInfo{}
	if err := c.Bind(vmInfoData); err != nil {
		return err
	}

	vmInfoData.Status = "Creating"

	//goroutin
	var wg sync.WaitGroup
	wg.Add(1)

	//CreateMcis(nsId, req)
	//err := AddVmToMcis(nsId, mcisId, vmInfoData)
	err := mcis.AddVmToMcis(&wg, nsId, mcisId, &vmInfoData)

	if err != nil {
		mapA := map[string]string{"message": "Cannot find " + common.GenMcisKey(nsId, mcisId, "")}
		return c.JSON(http.StatusOK, &mapA)
	}
	wg.Wait()

	vmStatus, err := mcis.GetVmStatus(nsId, mcisId, vmInfoData.Id)

	vmInfoData.Status = vmStatus.Status
	vmInfoData.TargetStatus = vmStatus.TargetStatus
	vmInfoData.TargetAction = vmStatus.TargetAction

	return c.JSON(http.StatusCreated, vmInfoData)
}

func RestGetMcisVm(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	action := c.QueryParam("action")
	fmt.Println("[Get VM requested action: " + action)
	if action == "suspend" {
		fmt.Println("[suspend VM]")

		mcis.ControlVm(nsId, mcisId, vmId, mcis.ActionSuspend)
		mapA := map[string]string{"message": "Suspending the VM"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "resume" {
		fmt.Println("[resume VM]")

		mcis.ControlVm(nsId, mcisId, vmId, mcis.ActionResume)
		mapA := map[string]string{"message": "Resuming the VM"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "reboot" {
		fmt.Println("[reboot VM]")

		mcis.ControlVm(nsId, mcisId, vmId, mcis.ActionReboot)
		mapA := map[string]string{"message": "Rebooting the VM"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "terminate" {
		fmt.Println("[terminate VM]")

		mcis.ControlVm(nsId, mcisId, vmId, mcis.ActionTerminate)

		mapA := map[string]string{"message": "Terminating the VM"}
		return c.JSON(http.StatusOK, &mapA)

	} else if action == "status" {

		fmt.Println("[status VM]")

		vmKey := common.GenMcisKey(nsId, mcisId, vmId)
		//fmt.Println(vmKey)
		vmKeyValue, _ := common.CBStore.Get(vmKey)
		if vmKeyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + vmKey}
			return c.JSON(http.StatusOK, &mapA)
		}

		vmStatusResponse, err := mcis.GetVmStatus(nsId, mcisId, vmId)

		if err != nil {
			common.CBLog.Error(err)
			return err
		}

		//fmt.Printf("%+v\n", vmStatusResponse)
		common.PrintJsonPretty(vmStatusResponse)

		return c.JSON(http.StatusOK, &vmStatusResponse)

	} else {

		fmt.Println("[Get MCIS-VM info for id]" + vmId)

		key := common.GenMcisKey(nsId, mcisId, "")
		//fmt.Println(key)

		vmKey := common.GenMcisKey(nsId, mcisId, vmId)
		//fmt.Println(vmKey)
		vmKeyValue, _ := common.CBStore.Get(vmKey)
		if vmKeyValue == nil {
			mapA := map[string]string{"message": "Cannot find " + key}
			return c.JSON(http.StatusOK, &mapA)
		}
		//fmt.Println("<" + vmKeyValue.Key + "> \n" + vmKeyValue.Value)
		vmTmp := mcis.TbVmInfo{}
		json.Unmarshal([]byte(vmKeyValue.Value), &vmTmp)
		vmTmp.Id = vmId

		//get current vm status
		vmStatusInfoTmp, err := mcis.GetVmStatus(nsId, mcisId, vmId)
		if err != nil {
			common.CBLog.Error(err)
		}

		vmTmp.Status = vmStatusInfoTmp.Status
		vmTmp.TargetStatus = vmStatusInfoTmp.TargetStatus
		vmTmp.TargetAction = vmStatusInfoTmp.TargetAction

		//fmt.Printf("%+v\n", vmTmp)
		common.PrintJsonPretty(vmTmp)

		//return by string
		//return c.String(http.StatusOK, keyValue.Value)
		return c.JSON(http.StatusOK, &vmTmp)

	}
}

func RestPutMcisVm(c echo.Context) error {
	return nil
}

func RestDelMcisVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	err := mcis.DelMcisVm(nsId, mcisId, vmId)
	if err != nil {
		common.CBLog.Error(err)
		mapA := map[string]string{"message": "Failed to delete the VM info"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	mapA := map[string]string{"message": "Deleting the VM info"}
	return c.JSON(http.StatusOK, &mapA)
}

func RestGetAllBenchmark(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	type bmReq struct {
		Host string `json:"host"`
	}
	req := &bmReq{}
	if err := c.Bind(req); err != nil {
		return err
	}
	target := req.Host

	action := "all"
	fmt.Println("[Get MCIS benchmark action: " + action + target)

	option := "localhost"
	option = target

	var err error

	content := mcis.BenchmarkInfoArray{}

	allBenchCmd := []string{"cpus", "cpum", "memR", "memW", "fioR", "fioW", "dbR", "dbW", "rtt"}

	resultMap := make(map[string]mcis.SpecBenchmarkInfo)

	for i, v := range allBenchCmd {
		fmt.Println("[Benchmark] " + v)
		content, err = mcis.BenchmarkAction(nsId, mcisId, v, option)
		for _, k := range content.ResultArray {
			SpecId := k.SpecId
			Result := k.Result
			specBenchInfoTmp := mcis.SpecBenchmarkInfo{}

			val, exist := resultMap[SpecId]
			if exist {
				specBenchInfoTmp = val
			} else {
				specBenchInfoTmp.SpecId = SpecId
			}

			switch i {
			case 0:
				specBenchInfoTmp.Cpus = Result
			case 1:
				specBenchInfoTmp.Cpum = Result
			case 2:
				specBenchInfoTmp.MemR = Result
			case 3:
				specBenchInfoTmp.MemW = Result
			case 4:
				specBenchInfoTmp.FioR = Result
			case 5:
				specBenchInfoTmp.FioW = Result
			case 6:
				specBenchInfoTmp.DbR = Result
			case 7:
				specBenchInfoTmp.DbW = Result
			case 8:
				specBenchInfoTmp.Rtt = Result
			}

			resultMap[SpecId] = specBenchInfoTmp

		}
	}

	file, err := os.OpenFile("benchmarking.csv", os.O_CREATE|os.O_WRONLY, 0777)
	defer file.Close()
	csvWriter := csv.NewWriter(file)
	strsTmp := []string{}
	for key, val := range resultMap {
		strsTmp = nil
		fmt.Println(key, val)
		strsTmp = append(strsTmp, val.SpecId)
		strsTmp = append(strsTmp, val.Cpus)
		strsTmp = append(strsTmp, val.Cpum)
		strsTmp = append(strsTmp, val.MemR)
		strsTmp = append(strsTmp, val.MemW)
		strsTmp = append(strsTmp, val.FioR)
		strsTmp = append(strsTmp, val.FioW)
		strsTmp = append(strsTmp, val.DbR)
		strsTmp = append(strsTmp, val.DbW)
		strsTmp = append(strsTmp, val.Rtt)
		csvWriter.Write(strsTmp)
		csvWriter.Flush()
	}

	file2, err := os.OpenFile("rttmap.csv", os.O_CREATE|os.O_WRONLY, 0777)
	defer file2.Close()
	csvWriter2 := csv.NewWriter(file2)

	const mrttArrayXMax = 50
	const mrttArrayYMax = 50
	mrttArray := make([][]string, mrttArrayXMax)
	for i := 0; i < mrttArrayXMax; i++ {
		mrttArray[i] = make([]string, mrttArrayYMax)
		for j := 0; j < mrttArrayYMax; j++ {
			mrttArray[i][j] = "0"
		}
	}

	rttIndexMapX := make(map[string]int)
	cntTargetX := 1
	rttIndexMapY := make(map[string]int)
	cntTargetY := 1

	action = "mrtt"
	fmt.Println("[Benchmark] " + action)
	content, err = mcis.BenchmarkAction(nsId, mcisId, action, option)
	for _, k := range content.ResultArray {
		SpecId := k.SpecId
		iX, exist := rttIndexMapX[SpecId]
		if !exist {
			rttIndexMapX[SpecId] = cntTargetX
			iX = cntTargetX
			mrttArray[iX][0] = SpecId
			cntTargetX++
		}
		for _, m := range k.ResultArray {
			tagetSpecId := m.SpecId
			tagetRtt := m.Result
			iY, exist2 := rttIndexMapY[tagetSpecId]
			if !exist2 {
				rttIndexMapY[tagetSpecId] = cntTargetY
				iY = cntTargetY
				mrttArray[0][iY] = tagetSpecId
				cntTargetY++
			}
			mrttArray[iX][iY] = tagetRtt
		}
	}

	csvWriter2.WriteAll(mrttArray)
	csvWriter2.Flush()

	if err != nil {
		mapError := map[string]string{"message": "Benchmark Error"}
		return c.JSON(http.StatusFailedDependency, &mapError)
	}
	common.PrintJsonPretty(content)
	return c.JSON(http.StatusOK, content)
}

func RestGetBenchmark(c echo.Context) error {
	//id, _ := strconv.Atoi(c.Param("id"))

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	type bmReq struct {
		Host string `json:"host"`
	}
	req := &bmReq{}
	if err := c.Bind(req); err != nil {
		return err
	}
	target := req.Host

	action := c.QueryParam("action")
	fmt.Println("[Get MCIS benchmark action: " + action + target)

	option := "localhost"
	option = target

	var err error
	content := mcis.BenchmarkInfoArray{}

	vaildActions := "install init cpus cpum memR memW fioR fioW dbR dbW rtt mrtt clean"

	fmt.Println("[Benchmark] " + action)
	if strings.Contains(vaildActions, action) {
		content, err = mcis.BenchmarkAction(nsId, mcisId, action, option)
	} else {
		mapA := map[string]string{"message": "Not available action"}
		return c.JSON(http.StatusFailedDependency, &mapA)
	}

	if err != nil {
		mapError := map[string]string{"message": "Benchmark Error"}
		return c.JSON(http.StatusFailedDependency, &mapError)
	}
	common.PrintJsonPretty(content)
	return c.JSON(http.StatusOK, content)
}
