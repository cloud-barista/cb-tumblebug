package mcis

import (

	"encoding/json"
	"fmt"
	"strings"


	"github.com/cloud-barista/cb-tumblebug/src/core/common"

)

// Status for mcis automation
const AutoStatusReady string = "Ready"
const AutoStatusChecking string = "Checking"
const AutoStatusDetected string = "Detected"
const AutoStatusOperating string = "Operating"
const AutoStatusTimeout string = "Timeout"
const AutoStatusError string = "Error"
const AutoStatusSuspended string = "Suspended"

// Action for mcis automation
const AutoActionScaleOut string = "ScaleOut"
const AutoActionScaleIn string = "ScaleIn"

type AutoCondition struct {
	Metric         string     `json:"metric"`
	Operator       string     `json:"operator"` // <, <=, >, >=, ...
	Operand        string     `json:"operand"`   // 10, 70, 80, 98, ...

	//InitTime	   string 	  `json:"initTime"`  // to check start of duration
	//Duration	   string 	  `json:"duration"`  // duration for checking 
}

type AutoAction struct {
	ActionType     string     `json:"actionType"`
	Vm             TbVmInfo   `json:"vm"`
	Placement_algo string     `json:"placement_algo"`
}

type Policy struct {
	AutoCondition     AutoCondition     `json:"autoCondition"`
	AutoAction        AutoAction   		`json:"autoAction"`
}

type McisPolicyInfo struct {
	Name         	string     `json:"Name"` //MCIS Name (for request)
	Id         		string     `json:"Id"`	 //MCIS Id (generated ID by the Name)
	Policy         []Policy   `json:"policy"`
	Status         string     `json:"status"`
	ActionLog	   string     `json:"actionLog"`
	Description    string     `json:"description"`
}

func ValidateStatus() {

	nsList := common.ListNsId()

	fmt.Println("")
	for _, nsId := range nsList {
		fmt.Println("NS[" + nsId + "]")
		mcisPolicyList := ListMcisPolicyId(nsId)

		for _, m := range mcisPolicyList {
			fmt.Println("validateStatus: MCIS-Policy[" + m + "]")
		}

		for _, v := range mcisPolicyList {

			key := common.GenMcisPolicyKey(nsId, v, "")
			//fmt.Println(key)
			keyValue, _ := common.CBStore.Get(key)
			if keyValue == nil {
				//mapA := map[string]string{"message": "Cannot find " + key}
				//return c.JSON(http.StatusOK, &mapA)
				fmt.Println("keyValue is nil")
			}
			//fmt.Println("<" + keyValue.Key + "> \n" + keyValue.Value)
			mcisPolicyTmp := McisPolicyInfo{}
			json.Unmarshal([]byte(keyValue.Value), &mcisPolicyTmp)

			/* FYI
			const AutoStatusReady string = "Ready"
			const AutoStatusChecking string = "Checking"
			const AutoStatusHappened string = "Happened"
			const AutoStatusOperating string = "Operating"
			const AutoStatusTimeout string = "Timeout"
			const AutoStatusError string = "Error"
			const AutoStatusSuspend string = "Suspend"
			*/
			fmt.Println("[MCIS-Policy-StateMachine]")
			switch {
				case mcisPolicyTmp.Status == AutoStatusReady:
					fmt.Println("MCIS-Policy-Status[" + AutoStatusReady + "],["+ v + "]")
					mcisPolicyTmp.Status = AutoStatusChecking
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
 					
					fmt.Println("[Check MCIS Policy] " + mcisPolicyTmp.Id)
					keyValueMcis, _ := common.CBStore.Get(common.GenMcisKey(nsId, mcisPolicyTmp.Id, ""))
					if keyValueMcis == nil {
						mcisPolicyTmp.Status = AutoStatusError
						UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
						break	
					} else { // need to enhance : loop for each policies and realize metric
						content, err := GetMonitoringData(nsId, mcisPolicyTmp.Id, mcisPolicyTmp.Policy[0].AutoCondition.Metric)
						if err != nil {
							common.CBLog.Error(err)
							mcisPolicyTmp.Status = AutoStatusError
							break
						}
						common.PrintJsonPretty(content)
					}

					mcisPolicyTmp.Status = AutoStatusReady

				case mcisPolicyTmp.Status == AutoStatusChecking:
					fmt.Println("MCIS-Policy-Status[" + AutoStatusChecking + "],["+ v + "]")
					mcisPolicyTmp.Status = AutoStatusDetected

				case mcisPolicyTmp.Status == AutoStatusDetected:
					fmt.Println("MCIS-Policy-Status[" + AutoStatusDetected + "],["+ v + "]")
					mcisPolicyTmp.Status = AutoStatusOperating

				case mcisPolicyTmp.Status == AutoStatusOperating:
					fmt.Println("MCIS-Policy-Status[" + AutoStatusOperating + "],["+ v + "]")
					mcisPolicyTmp.Status = AutoStatusReady

				case mcisPolicyTmp.Status == AutoStatusTimeout:
					fmt.Println("MCIS-Policy-Status[" + AutoStatusTimeout + "],["+ v + "]")

				case mcisPolicyTmp.Status == AutoStatusError:
					fmt.Println("MCIS-Policy-Status[" + AutoStatusError + "],["+ v + "]")
					mcisPolicyTmp.Status = AutoStatusReady

				case mcisPolicyTmp.Status == AutoStatusSuspended:
					fmt.Println("MCIS-Policy-Status[" + AutoStatusSuspended + "],["+ v + "]")

				default:
			}
			UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
	
		}

	}
	

}


func UpdateMcisPolicyInfo(nsId string, mcisPolicyInfoData McisPolicyInfo) {
	key := common.GenMcisPolicyKey(nsId, mcisPolicyInfoData.Id, "")
	val, _ := json.Marshal(mcisPolicyInfoData)
	err := common.CBStore.Put(string(key), string(val))
	if err != nil {
		common.CBLog.Error(err)
	}
	//fmt.Println("===========================")
	//vmkeyValue, _ := common.CBStore.Get(string(key))
	//fmt.Println("<" + vmkeyValue.Key + "> \n" + vmkeyValue.Value)
	//fmt.Println("===========================")
}


func CreateMcisPolicy(nsId string, mcisId string, u *McisPolicyInfo) (McisPolicyInfo, error) {

	nsId = common.GenId(nsId)
	check, lowerizedName, _ := LowerizeAndCheckMcisPolicy(nsId, mcisId)
	//fmt.Println("CreateVNet() called; nsId: " + nsId + ", u.Name: " + u.Name + ", lowerizedName: " + lowerizedName) // for debug
	u.Name = lowerizedName
	u.Id = lowerizedName
	u.Status = AutoStatusReady

	if check == true {
		temp := McisPolicyInfo{}
		err := fmt.Errorf("The MCIS Policy Obj " + u.Name + " already exists.")
		return temp, err
	}

	content := *u

	// cb-store
	fmt.Println("=========================== PUT CreateMcisPolicy")
	Key := common.GenMcisPolicyKey(nsId, content.Id, "")
	Val, _ := json.Marshal(content)

	//fmt.Println("Key: ", Key)
	//fmt.Println("Val: ", Val)
	err := common.CBStore.Put(string(Key), string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return content, err
	}
	keyValue, _ := common.CBStore.Get(string(Key))
	fmt.Println("<KEY>\n" + keyValue.Key + "\n<VAL>\n" + keyValue.Value)
	fmt.Println("===========================")

	return content, nil
}

func GetMcisPolicyObject(nsId string, mcisId string) (McisPolicyInfo, error) {
	fmt.Println("[GetMcisPolicyObject]" + mcisId)
	nsId = common.GenId(nsId)
	key := common.GenMcisPolicyKey(nsId, mcisId, "")
	fmt.Println("Key: ", key)
	keyValue, err := common.CBStore.Get(key)
	if err != nil {
		common.CBLog.Error(err)
		return McisPolicyInfo{}, err
	}
	if keyValue == nil {
		return McisPolicyInfo{}, err
	}

	fmt.Println("<KEY>\n" + keyValue.Key + "\n<VAL>\n" + keyValue.Value)

	mcisPolicyTmp := McisPolicyInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mcisPolicyTmp)
	return mcisPolicyTmp, nil
}


func GetAllMcisPolicyObject(nsId string) ([]McisPolicyInfo, error) {

	nsId = common.GenId(nsId)
	Mcis := []McisPolicyInfo{}
	mcisList := ListMcisPolicyId(nsId)

	for _, v := range mcisList {

		key := common.GenMcisPolicyKey(nsId, v, "")
		keyValue, _ := common.CBStore.Get(key)
		if keyValue == nil {
			return nil, fmt.Errorf("Cannot find " + key)
		}
		mcisTmp := McisPolicyInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
		Mcis = append(Mcis, mcisTmp)
	}

	return Mcis, nil
}


func ListMcisPolicyId(nsId string) []string {

	nsId = common.GenId(nsId)
	fmt.Println("[Get MCIS Policy ID list]")
	key := "/ns/" + nsId + "/policy/mcis"
	keyValue, _ := common.CBStore.GetList(key, true)

	var mcisList []string
	for _, v := range keyValue {
		if !strings.Contains(v.Key, "vm") {
			mcisList = append(mcisList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/policy/mcis/"))
		}
	}
	return mcisList
}

func DelMcisPolicy(nsId string, mcisId string) error {

	nsId = common.GenId(nsId)
	check, lowerizedName, _ := LowerizeAndCheckMcisPolicy(nsId, mcisId)
	mcisId = lowerizedName

	if check == false {
		err := fmt.Errorf("The mcis Policy" + mcisId + " does not exist.")
		return err
	}

	fmt.Println("[Delete MCIS Policy] " + mcisId)

	key := common.GenMcisPolicyKey(nsId, mcisId, "")
	fmt.Println(key)

	// delete mcis Policy info
	err := common.CBStore.Delete(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return nil
}

func DelAllMcisPolicy(nsId string) (string, error) {

	nsId = common.GenId(nsId)
	mcisList := ListMcisPolicyId(nsId)
	if len(mcisList) == 0 {
		return "No MCIS Policy to delete", nil
	}
	for _, v := range mcisList {
		err := DelMcisPolicy(nsId, v)
		if err != nil {
			common.CBLog.Error(err)

			return "", fmt.Errorf("Failed to delete All MCISs")
		}
	}
	return "All MCIS Policys has been deleted", nil
}
