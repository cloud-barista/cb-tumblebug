/*
Copyright 2019 The Cloud-Barista Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package mcis is to manage multi-cloud infra service
package mcis

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
)

// Status for mcis automation
const (
	// AutoStatusReady is const for "Ready" status.
	AutoStatusReady string = "Ready"

	// AutoStatusChecking is const for "Checking" status.
	AutoStatusChecking string = "Checking"

	// AutoStatusDetected is const for "Detected" status.
	AutoStatusDetected string = "Detected"

	// AutoStatusOperating is const for "Operating" status.
	AutoStatusOperating string = "Operating"

	// AutoStatusStabilizing is const for "Stabilizing" status.
	AutoStatusStabilizing string = "Stabilizing"

	// AutoStatusTimeout is const for "Timeout" status.
	AutoStatusTimeout string = "Timeout"

	// AutoStatusError is const for "Failed" status.
	AutoStatusError string = "Failed"

	// AutoStatusSuspended is const for "Suspended" status.
	AutoStatusSuspended string = "Suspended"
)

// Action for mcis automation
const (
	// AutoActionScaleOut is const for "ScaleOut" action.
	AutoActionScaleOut string = "ScaleOut"

	// AutoActionScaleIn is const for "ScaleIn" action.
	AutoActionScaleIn string = "ScaleIn"
)

// AutoCondition is struct for MCIS auto-control condition.
type AutoCondition struct {
	Metric           string   `json:"metric" example:"cpu"`
	Operator         string   `json:"operator" example:">=" enums:"<,<=,>,>="`
	Operand          string   `json:"operand" example:"80"`
	EvaluationPeriod string   `json:"evaluationPeriod" example:"10"`
	EvaluationValue  []string `json:"evaluationValue"`
	//InitTime	   string 	  `json:"initTime"`  // to check start of duration
	//Duration	   string 	  `json:"duration"`  // duration for checking
}

// AutoAction is struct for MCIS auto-control action.
type AutoAction struct {
	ActionType   string         `json:"actionType" example:"ScaleOut" enums:"ScaleOut,ScaleIn"`
	VmDynamicReq TbVmDynamicReq `json:"vmDynamicReq"`

	// PostCommand is field for providing command to VMs after its creation. example:"wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setweb.sh -O ~/setweb.sh; chmod +x ~/setweb.sh; sudo ~/setweb.sh"
	PostCommand   McisCmdReq `json:"postCommand"`
	PlacementAlgo string     `json:"placementAlgo" example:"random"`
}

// Policy is struct for MCIS auto-control Policy request that includes AutoCondition, AutoAction, Status.
type Policy struct {
	AutoCondition AutoCondition `json:"autoCondition"`
	AutoAction    AutoAction    `json:"autoAction"`
	Status        string        `json:"status"`
}

// McisPolicyInfo is struct for MCIS auto-control Policy object.
type McisPolicyInfo struct {
	Name   string   `json:"Name"` //MCIS Name (for request)
	Id     string   `json:"Id"`   //MCIS Id (generated ID by the Name)
	Policy []Policy `json:"policy"`

	ActionLog   string `json:"actionLog"`
	Description string `json:"description" example:"Description"`
}

// McisPolicyReq is struct for MCIS auto-control Policy Request.
type McisPolicyReq struct {
	Policy      []Policy `json:"policy"`
	Description string   `json:"description" example:"Description"`
}

// OrchestrationController is responsible for executing MCIS automation policy.
// OrchestrationController will be periodically involked by a time.NewTicker in main.go.
func OrchestrationController() {

	nsList, err := common.ListNsId()
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("an error occurred while getting namespaces' list: " + err.Error())
		return
	}

	for _, nsId := range nsList {

		mcisPolicyList := ListMcisPolicyId(nsId)

		for _, m := range mcisPolicyList {
			fmt.Println("NS[" + nsId + "]" + "McisPolicy[" + m + "]")
		}

		for _, v := range mcisPolicyList {

			key := common.GenMcisPolicyKey(nsId, v, "")
			keyValue, err := common.CBStore.Get(key)
			if err != nil {
				common.CBLog.Error(err)
				err = fmt.Errorf("In OrchestrationController(); CBStore.Get() returned an error.")
				common.CBLog.Error(err)
				// return nil, err
			}

			if keyValue == nil {
				fmt.Println("keyValue is nil")
			}
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

			for policyIndex := range mcisPolicyTmp.Policy {
				fmt.Println("\n[MCIS-Policy-StateMachine]")
				common.PrintJsonPretty(mcisPolicyTmp.Policy[policyIndex])

				switch {
				case mcisPolicyTmp.Policy[policyIndex].Status == AutoStatusReady:
					fmt.Println("- PolicyStatus[" + AutoStatusReady + "],[" + v + "]")
					mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusChecking
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)

					fmt.Println("[Check MCIS Policy] " + mcisPolicyTmp.Id)
					check, err := CheckMcis(nsId, mcisPolicyTmp.Id)
					fmt.Println("[Check existence of MCIS] " + mcisPolicyTmp.Id)
					//keyValueMcis, _ := common.CBStore.Get(common.GenMcisKey(nsId, mcisPolicyTmp.Id, ""))

					if !check || err != nil {
						mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusError
						UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
						fmt.Println("[MCIS is not exist] " + mcisPolicyTmp.Id)
						break
					} else { // need to enhance : loop for each policies and realize metric

						//Checking (measuring)
						mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusChecking
						UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
						fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

						fmt.Println("[MCIS is exist] " + mcisPolicyTmp.Id)
						content, err := GetMonitoringData(nsId, mcisPolicyTmp.Id, mcisPolicyTmp.Policy[policyIndex].AutoCondition.Metric)
						if err != nil {
							common.CBLog.Error(err)
							mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusError
							break
						}

						//Statistic
						sumMcis := 0.0
						for _, monData := range content.McisMonitoring {
							monDataValue, _ := strconv.ParseFloat(monData.Value, 64)
							sumMcis += monDataValue
						}
						averMcis := (sumMcis / float64(len(content.McisMonitoring)))
						fmt.Printf("[monData.Value] AverMcis: %f,  SumMcis: %f \n", averMcis, sumMcis)

						evaluationPeriod, _ := strconv.Atoi(mcisPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationPeriod)
						evaluationValue := mcisPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationValue
						evaluationValue = append([]string{fmt.Sprintf("%f", averMcis)}, evaluationValue...) // prepend current aver date
						mcisPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationValue = evaluationValue

						sum := 0.0
						aver := -0.1
						// accumerate previous evaluation value
						fmt.Printf("[Evaluation History]\n")
						for evi, evv := range evaluationValue {
							evvFloat, _ := strconv.ParseFloat(evv, 64)
							sum += evvFloat
							fmt.Printf("[%v] %f ", evi, evvFloat)
							// break with outside evaluationValue
							if evi >= evaluationPeriod-1 {
								break
							}
						}
						// average for evaluationPeriod (if data for the period is not enough, skip)
						if evaluationPeriod != 0 && len(evaluationValue) >= evaluationPeriod {
							aver = sum / float64(evaluationPeriod)
						}
						fmt.Printf("\n[Evaluation] Aver: %f,  Period: %v \n", aver, evaluationPeriod)

						//Detecting
						operator := mcisPolicyTmp.Policy[policyIndex].AutoCondition.Operator
						operand, _ := strconv.ParseFloat(mcisPolicyTmp.Policy[policyIndex].AutoCondition.Operand, 64)

						if evaluationPeriod == 0 {
							fmt.Println("[Checking] Not available evaluationPeriod ")
							mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusError
							UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
							break
						}
						// not enough evaluationPeriod
						if aver == -0.1 {
							fmt.Println("[Checking] Not enough evaluationPeriod ")
							mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
							UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
							break
						}
						switch {
						case operator == ">=":
							if aver >= operand {
								fmt.Printf("[Detected] Aver: %f >=  Operand: %f \n", aver, operand)
								mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusDetected
							} else {
								fmt.Printf("[Not Detected] Aver: %f >=  Operand: %f \n", aver, operand)
								mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
							}
						case operator == ">":
							if aver > operand {
								fmt.Printf("[Detected] Aver: %f >  Operand: %f \n", aver, operand)
								mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f >  Operand: %f \n", aver, operand)
								mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
							}
						case operator == "<=":
							if aver <= operand {
								fmt.Printf("[Detected] Aver: %f <=  Operand: %f \n", aver, operand)
								mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f <=  Operand: %f \n", aver, operand)
								mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
							}
						case operator == "<":
							if aver < operand {
								fmt.Printf("[Detected] Aver: %f <  Operand: %f \n", aver, operand)
								mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f <  Operand: %f \n", aver, operand)
								mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
							}
						default:
							fmt.Println("[Checking] Not available operator " + operator)
							mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusError
						}
					}
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case mcisPolicyTmp.Policy[policyIndex].Status == AutoStatusChecking:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					//mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusDetected

				case mcisPolicyTmp.Policy[policyIndex].Status == AutoStatusDetected:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusOperating
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

					//Action
					/*
						// Actions for mcis automation
						const AutoActionScaleOut string = "ScaleOut"
						const AutoActionScaleIn string = "ScaleIn"
					*/

					autoAction := mcisPolicyTmp.Policy[policyIndex].AutoAction
					fmt.Println("[autoAction] " + autoAction.ActionType)

					switch {
					case autoAction.ActionType == AutoActionScaleOut:

						autoAction.VmDynamicReq.Label = labelAutoGen
						// append UUID to given vm name to avoid duplicated vm ID.
						autoAction.VmDynamicReq.Name = common.ToLower(autoAction.VmDynamicReq.Name) + "-" + common.GenUid()
						//vmReqTmp := autoAction.Vm
						// autoAction.VmDynamicReq.SubGroupSize = "1"

						if autoAction.PlacementAlgo == "random" {
							fmt.Println("[autoAction.PlacementAlgo] " + autoAction.PlacementAlgo)
							// var vmTmpErr error
							// existingVm, vmTmpErr := GetVmTemplate(nsId, mcisPolicyTmp.Id, autoAction.PlacementAlgo)
							// if vmTmpErr != nil {
							// 	mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusError
							// 	UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
							// }

							autoAction.VmDynamicReq.CommonImage = "ubuntu18.04"                // temporal default value. will be changed
							autoAction.VmDynamicReq.CommonSpec = "aws-ap-northeast-2-t2-small" // temporal default value. will be changed

							deploymentPlan := DeploymentPlan{}

							deploymentPlan.Priority.Policy = append(deploymentPlan.Priority.Policy, PriorityCondition{Metric: "random"})
							specList, err := RecommendVm(common.SystemCommonNs, deploymentPlan)
							if err != nil {
								mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusError
								UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
							}
							if len(specList) != 0 {
								recommendedSpec := specList[0].Id
								autoAction.VmDynamicReq.CommonSpec = recommendedSpec
							}

							// autoAction.VmDynamicReq.Name = autoAction.VmDynamicReq.Name + "-random"
							// autoAction.VmDynamicReq.Label = labelAutoGen
						}

						common.PrintJsonPretty(autoAction.VmDynamicReq)
						fmt.Println("[Action] " + autoAction.ActionType)

						// ScaleOut MCIS according to the VM requirement.
						fmt.Println("[Generating VM]")
						result, vmCreateErr := CreateMcisVmDynamic(nsId, mcisPolicyTmp.Id, &autoAction.VmDynamicReq)
						if vmCreateErr != nil {
							mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusError
							UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
						}
						common.PrintJsonPretty(result)

						nullMcisCmdReq := McisCmdReq{}
						if autoAction.PostCommand != nullMcisCmdReq {
							fmt.Println("[Post Command to VM] " + autoAction.PostCommand.Command)
							_, cmdErr := RemoteCommandToMcis(nsId, mcisPolicyTmp.Id, common.ToLower(autoAction.VmDynamicReq.Name), &autoAction.PostCommand)
							if cmdErr != nil {
								mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusError
								UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
							}
						}

					case autoAction.ActionType == AutoActionScaleIn:
						fmt.Println("[Action] " + autoAction.ActionType)

						// ScaleIn MCIS.
						fmt.Println("[Removing VM]")
						vmList, vmListErr := GetVmListByLabel(nsId, mcisPolicyTmp.Id, labelAutoGen)
						if vmListErr != nil {
							mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusError
							UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
						}
						if len(vmList) != 0 {
							removeTargetVm := vmList[len(vmList)-1]
							fmt.Println("[Removing VM ID] " + removeTargetVm)
							delVmErr := DelMcisVm(nsId, mcisPolicyTmp.Id, removeTargetVm, "")
							if delVmErr != nil {
								mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusError
								UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
							}
						}

					default:
					}

					mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusStabilizing
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case mcisPolicyTmp.Policy[policyIndex].Status == AutoStatusStabilizing:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

					//initialize Evaluation history so that controller does not act too early.
					//with this we can stablize MCIS by init previously measures.
					//Will invoke [Checking] Not enough evaluationPeriod
					mcisPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationValue = nil

					mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)

				case mcisPolicyTmp.Policy[policyIndex].Status == AutoStatusOperating:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					//mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
					//UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)

				case mcisPolicyTmp.Policy[policyIndex].Status == AutoStatusTimeout:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case mcisPolicyTmp.Policy[policyIndex].Status == AutoStatusError:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					mcisPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)

				case mcisPolicyTmp.Policy[policyIndex].Status == AutoStatusSuspended:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				default:
				}
			}

		}

	}

}

// UpdateMcisPolicyInfo updates McisPolicyInfo object in DB.
func UpdateMcisPolicyInfo(nsId string, mcisPolicyInfoData McisPolicyInfo) {
	key := common.GenMcisPolicyKey(nsId, mcisPolicyInfoData.Id, "")
	val, _ := json.Marshal(mcisPolicyInfoData)
	err := common.CBStore.Put(key, string(val))
	if err != nil && !strings.Contains(err.Error(), common.CbStoreKeyNotFoundErrorString) {
		common.CBLog.Error(err)
	}
}

// CreateMcisPolicy create McisPolicyInfo object in DB according to user's requirements.
func CreateMcisPolicy(nsId string, mcisId string, u *McisPolicyReq) (McisPolicyInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := McisPolicyInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := McisPolicyInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
	check, _ := CheckMcisPolicy(nsId, mcisId)

	//u.Status = AutoStatusReady

	if check {
		temp := McisPolicyInfo{}
		err := fmt.Errorf("The MCIS Policy Obj " + mcisId + " already exists.")
		return temp, err
	}

	for policyIndex := range u.Policy {
		u.Policy[policyIndex].Status = AutoStatusReady
	}

	req := *u
	obj := McisPolicyInfo{}
	obj.Name = mcisId
	obj.Id = mcisId
	obj.Policy = req.Policy
	obj.Description = req.Description

	// cb-store
	Key := common.GenMcisPolicyKey(nsId, obj.Id, "")
	Val, _ := json.Marshal(obj)

	err = common.CBStore.Put(Key, string(Val))
	if err != nil {
		common.CBLog.Error(err)
		return obj, err
	}
	keyValue, err := common.CBStore.Get(Key)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In CreateMcisPolicy(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	fmt.Println("<KEY>\n" + keyValue.Key + "\n<VAL>\n" + keyValue.Value)

	return obj, nil
}

// GetMcisPolicyObject returns McisPolicyInfo object.
func GetMcisPolicyObject(nsId string, mcisId string) (McisPolicyInfo, error) {
	fmt.Println("[GetMcisPolicyObject]" + mcisId)

	err := common.CheckString(nsId)
	if err != nil {
		temp := McisPolicyInfo{}
		common.CBLog.Error(err)
		return temp, err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		temp := McisPolicyInfo{}
		common.CBLog.Error(err)
		return temp, err
	}
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

// GetAllMcisPolicyObject returns all McisPolicyInfo objects.
func GetAllMcisPolicyObject(nsId string) ([]McisPolicyInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil, err
	}
	Mcis := []McisPolicyInfo{}
	mcisList := ListMcisPolicyId(nsId)

	for _, v := range mcisList {

		key := common.GenMcisPolicyKey(nsId, v, "")
		keyValue, err := common.CBStore.Get(key)
		if err != nil {
			common.CBLog.Error(err)
			err = fmt.Errorf("In GetAllMcisPolicyObject(); CBStore.Get() returned an error.")
			common.CBLog.Error(err)
			// return nil, err
		}

		if keyValue == nil {
			return nil, fmt.Errorf("Cannot find " + key)
		}
		mcisTmp := McisPolicyInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mcisTmp)
		Mcis = append(Mcis, mcisTmp)
	}

	return Mcis, nil
}

// ListMcisPolicyId returns a list of Ids for all McisPolicyInfo objects .
func ListMcisPolicyId(nsId string) []string {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return nil
	}

	key := "/ns/" + nsId + "/policy/mcis"
	keyValue, err := common.CBStore.GetList(key, true)
	if err != nil {
		common.CBLog.Error(err)
		err = fmt.Errorf("In ListMcisPolicyId(); CBStore.Get() returned an error.")
		common.CBLog.Error(err)
		// return nil, err
	}

	var mcisList []string
	for _, v := range keyValue {
		if !strings.Contains(v.Key, "vm") {
			mcisList = append(mcisList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/policy/mcis/"))
		}
	}
	return mcisList
}

// DelMcisPolicy deletes McisPolicyInfo object by mcisId.
func DelMcisPolicy(nsId string, mcisId string) error {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	err = common.CheckString(mcisId)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}
	check, _ := CheckMcisPolicy(nsId, mcisId)

	if !check {
		err := fmt.Errorf("The mcis Policy" + mcisId + " does not exist.")
		return err
	}

	fmt.Println("[Delete MCIS Policy] " + mcisId)

	key := common.GenMcisPolicyKey(nsId, mcisId, "")
	fmt.Println(key)

	// delete mcis Policy info
	err = common.CBStore.Delete(key)
	if err != nil {
		common.CBLog.Error(err)
		return err
	}

	return nil
}

// DelAllMcisPolicy deletes all McisPolicyInfo objects.
func DelAllMcisPolicy(nsId string) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		common.CBLog.Error(err)
		return "", err
	}
	mcisList := ListMcisPolicyId(nsId)
	if len(mcisList) == 0 {
		return "No MCIS Policy to delete", nil
	}
	for _, v := range mcisList {
		err := DelMcisPolicy(nsId, v)
		if err != nil {
			common.CBLog.Error(err)

			return "", fmt.Errorf("Failed to delete All MCIS Policies")
		}
	}
	return "All MCIS Policies has been deleted", nil
}
