package mcis

import (
	//"encoding/json"
	//uuid "github.com/google/uuid"
	"fmt"
	"sync"

	//"fmt"
	//"net/http"
	//"io/ioutil"
	//"strconv"

	// CB-Store

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	//"github.com/cloud-barista/cb-spider/cloud-control-manager/vm-ssh"
	//"github.com/cloud-barista/cb-tumblebug/src/core/mcism"
	//"github.com/cloud-barista/cb-tumblebug/src/core/common"
)

// CB-Store
//var cblog *logrus.Logger
//var store icbs.Store

//var SPIDER_REST_URL string

func init() {
	//cblog = config.Cblogger
	//store = cbstore.GetStore()
	//SPIDER_REST_URL = os.Getenv("SPIDER_REST_URL")
}

/*
func genUuid() string {
	return uuid.New().String()
}
*/

/*
type mcirIds struct {
	CspImageId           string
	CspImageName         string
	CspSshKeyName        string
	Name                 string // Spec
	CspVNetId            string
	CspVNetName          string
	CspSecurityGroupId   string
	CspSecurityGroupName string
	CspPublicIpId        string
	CspPublicIpName      string
	CspVNicId            string
	CspVNicName          string

	ConnectionName string
}
*/

func LowerizeAndCheckMcis(nsId string, mcisId string) (bool, string, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckMcis failed; nsId given is null.")
		return false, "", err
	} else if mcisId == "" {
		err := fmt.Errorf("CheckMcis failed; mcisId given is null.")
		return false, "", err
	}

	lowerizedNsId := common.GenId(nsId)
	nsId = lowerizedNsId

	lowerizedMcisId := common.GenId(mcisId)
	mcisId = lowerizedMcisId

	fmt.Println("[Check mcis] " + mcisId)

	//key := "/ns/" + nsId + "/mcis/" + mcisId
	key := common.GenMcisKey(nsId, mcisId, "")
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
	/*
		if err != nil {
			common.CBLog.Error(err)
			return false, mcisId, err
		}
	*/
	if keyValue != nil {
		return true, mcisId, nil
	}
	return false, mcisId, nil

}

func LowerizeAndCheckVm(nsId string, mcisId string, vmId string) (bool, string, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("CheckVm failed; nsId given is null.")
		return false, "", err
	} else if mcisId == "" {
		err := fmt.Errorf("CheckVm failed; mcisId given is null.")
		return false, "", err
	} else if vmId == "" {
		err := fmt.Errorf("CheckVm failed; vmId given is null.")
		return false, "", err
	}

	lowerizedNsId := common.GenId(nsId)
	nsId = lowerizedNsId

	lowerizedMcisId := common.GenId(mcisId)
	mcisId = lowerizedMcisId

	lowerizedVmId := common.GenId(vmId)
	vmId = lowerizedVmId

	fmt.Println("[Check vm] " + mcisId + ", " + vmId)

	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, _ := common.CBStore.Get(key)
	/*
		if err != nil {
			common.CBLog.Error(err)
			return false, lowerizedVmId, err
		}
	*/
	if keyValue != nil {
		return true, lowerizedVmId, nil
	}
	return false, lowerizedVmId, nil

}

func RunSSH(vmIP string, userName string, privateKey string, cmd string) (*string, error) {

	// VM SSH 접속정보 설정 (외부 연결 정보, 사용자 아이디, Private Key)
	serverEndpoint := fmt.Sprintf("%s:22", vmIP)
	sshInfo := SSHInfo{
		ServerPort: serverEndpoint,
		UserName:   userName,
		PrivateKey: []byte(privateKey),
	}

	// VM SSH 명령어 실행
	if result, err := SSHRun(sshInfo, cmd); err != nil {
		return nil, err
	} else {
		return &result, nil
	}
}

func RunSSHAsync(wg *sync.WaitGroup, vmID string, vmIP string, userName string, privateKey string, cmd string, returnResult *[]SshCmdResult) {

	defer wg.Done() //goroutin sync done

	// VM SSH 접속정보 설정 (외부 연결 정보, 사용자 아이디, Private Key)
	serverEndpoint := fmt.Sprintf("%s:22", vmIP)
	sshInfo := SSHInfo{
		ServerPort: serverEndpoint,
		UserName:   userName,
		PrivateKey: []byte(privateKey),
	}

	// VM SSH 명령어 실행
	result, err := SSHRun(sshInfo, cmd)

	//wg.Done() //goroutin sync done

	sshResultTmp := SshCmdResult{}
	sshResultTmp.Mcis_id = ""
	sshResultTmp.Vm_id = vmID
	sshResultTmp.Vm_ip = vmIP

	if err != nil {
		sshResultTmp.Result = err.Error()
		sshResultTmp.Err = err
		*returnResult = append(*returnResult, sshResultTmp)
	} else {
		fmt.Println("cmd result " + result)
		sshResultTmp.Result = result
		sshResultTmp.Err = nil
		*returnResult = append(*returnResult, sshResultTmp)
	}

}
