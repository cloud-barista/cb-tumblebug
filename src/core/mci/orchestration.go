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

// Package mci is to manage multi-cloud infra service
package mci

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

// Status for mci automation
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

// Action for mci automation
const (
	// AutoActionScaleOut is const for "ScaleOut" action.
	AutoActionScaleOut string = "ScaleOut"

	// AutoActionScaleIn is const for "ScaleIn" action.
	AutoActionScaleIn string = "ScaleIn"
)

// AutoCondition is struct for MCI auto-control condition.
type AutoCondition struct {
	Metric           string   `json:"metric" example:"cpu"`
	Operator         string   `json:"operator" example:">=" enums:"<,<=,>,>="`
	Operand          string   `json:"operand" example:"80"`
	EvaluationPeriod string   `json:"evaluationPeriod" example:"10"`
	EvaluationValue  []string `json:"evaluationValue"`
	//InitTime	   string 	  `json:"initTime"`  // to check start of duration
	//Duration	   string 	  `json:"duration"`  // duration for checking
}

// AutoAction is struct for MCI auto-control action.
type AutoAction struct {
	ActionType   string         `json:"actionType" example:"ScaleOut" enums:"ScaleOut,ScaleIn"`
	VmDynamicReq TbVmDynamicReq `json:"vmDynamicReq"`

	// PostCommand is field for providing command to VMs after its creation. example:"wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setweb.sh -O ~/setweb.sh; chmod +x ~/setweb.sh; sudo ~/setweb.sh"
	PostCommand   MciCmdReq `json:"postCommand"`
	PlacementAlgo string    `json:"placementAlgo" example:"random"`
}

// Policy is struct for MCI auto-control Policy request that includes AutoCondition, AutoAction, Status.
type Policy struct {
	AutoCondition AutoCondition `json:"autoCondition"`
	AutoAction    AutoAction    `json:"autoAction"`
	Status        string        `json:"status"`
}

// MciPolicyInfo is struct for MCI auto-control Policy object.
type MciPolicyInfo struct {
	Name   string   `json:"Name"` //MCI Name (for request)
	Id     string   `json:"Id"`   //MCI Id (generated ID by the Name)
	Policy []Policy `json:"policy"`

	ActionLog   string `json:"actionLog"`
	Description string `json:"description" example:"Description"`
}

// MciPolicyReq is struct for MCI auto-control Policy Request.
type MciPolicyReq struct {
	Policy      []Policy `json:"policy"`
	Description string   `json:"description" example:"Description"`
}

// OrchestrationController is responsible for executing MCI automation policy.
// OrchestrationController will be periodically involked by a time.NewTicker in main.go.
func OrchestrationController() {

	nsList, err := common.ListNsId()
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("an error occurred while getting namespaces' list: " + err.Error())
		return
	}

	for _, nsId := range nsList {

		mciPolicyList := ListMciPolicyId(nsId)

		for _, m := range mciPolicyList {
			log.Debug().Msg("NS[" + nsId + "]" + "MciPolicy[" + m + "]")
		}

		for _, v := range mciPolicyList {

			key := common.GenMciPolicyKey(nsId, v, "")
			keyValue, err := kvstore.GetKv(key)
			if err != nil {
				log.Error().Err(err).Msg("")
				err = fmt.Errorf("In OrchestrationController(); kvstore.GetKv() returned an error.")
				log.Error().Err(err).Msg("")
				// return nil, err
			}

			if keyValue == (kvstore.KeyValue{}) {
				log.Debug().Msg("keyValue is nil")
			}
			mciPolicyTmp := MciPolicyInfo{}
			json.Unmarshal([]byte(keyValue.Value), &mciPolicyTmp)

			/* FYI
			const AutoStatusReady string = "Ready"
			const AutoStatusChecking string = "Checking"
			const AutoStatusHappened string = "Happened"
			const AutoStatusOperating string = "Operating"
			const AutoStatusTimeout string = "Timeout"
			const AutoStatusError string = "Error"
			const AutoStatusSuspend string = "Suspend"
			*/

			for policyIndex := range mciPolicyTmp.Policy {
				log.Debug().Msg("\n[MCI-Policy-StateMachine]")
				common.PrintJsonPretty(mciPolicyTmp.Policy[policyIndex])

				switch {
				case mciPolicyTmp.Policy[policyIndex].Status == AutoStatusReady:
					log.Debug().Msg("- PolicyStatus[" + AutoStatusReady + "],[" + v + "]")
					mciPolicyTmp.Policy[policyIndex].Status = AutoStatusChecking
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)

					log.Debug().Msg("[Check MCI Policy] " + mciPolicyTmp.Id)
					check, err := CheckMci(nsId, mciPolicyTmp.Id)
					log.Debug().Msg("[Check existence of MCI] " + mciPolicyTmp.Id)
					//keyValueMci, _ := common.Ckvstore.GetKv(common.GenMciKey(nsId, mciPolicyTmp.Id, ""))

					if !check || err != nil {
						mciPolicyTmp.Policy[policyIndex].Status = AutoStatusError
						UpdateMciPolicyInfo(nsId, mciPolicyTmp)
						log.Debug().Msg("[MCI is not exist] " + mciPolicyTmp.Id)
						break
					} else { // need to enhance : loop for each policies and realize metric

						//Checking (measuring)
						mciPolicyTmp.Policy[policyIndex].Status = AutoStatusChecking
						UpdateMciPolicyInfo(nsId, mciPolicyTmp)
						log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

						log.Debug().Msg("[MCI is exist] " + mciPolicyTmp.Id)
						content, err := GetMonitoringData(nsId, mciPolicyTmp.Id, mciPolicyTmp.Policy[policyIndex].AutoCondition.Metric)
						if err != nil {
							log.Error().Err(err).Msg("")
							mciPolicyTmp.Policy[policyIndex].Status = AutoStatusError
							break
						}

						//Statistic
						sumMci := 0.0
						for _, monData := range content.MciMonitoring {
							monDataValue, _ := strconv.ParseFloat(monData.Value, 64)
							sumMci += monDataValue
						}
						averMci := (sumMci / float64(len(content.MciMonitoring)))
						fmt.Printf("[monData.Value] AverMci: %f,  SumMci: %f \n", averMci, sumMci)

						evaluationPeriod, _ := strconv.Atoi(mciPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationPeriod)
						evaluationValue := mciPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationValue
						evaluationValue = append([]string{fmt.Sprintf("%f", averMci)}, evaluationValue...) // prepend current aver date
						mciPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationValue = evaluationValue

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
						operator := mciPolicyTmp.Policy[policyIndex].AutoCondition.Operator
						operand, _ := strconv.ParseFloat(mciPolicyTmp.Policy[policyIndex].AutoCondition.Operand, 64)

						if evaluationPeriod == 0 {
							log.Debug().Msg("[Checking] Not available evaluationPeriod ")
							mciPolicyTmp.Policy[policyIndex].Status = AutoStatusError
							UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							break
						}
						// not enough evaluationPeriod
						if aver == -0.1 {
							log.Debug().Msg("[Checking] Not enough evaluationPeriod ")
							mciPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
							UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							break
						}
						switch {
						case operator == ">=":
							if aver >= operand {
								fmt.Printf("[Detected] Aver: %f >=  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = AutoStatusDetected
							} else {
								fmt.Printf("[Not Detected] Aver: %f >=  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
							}
						case operator == ">":
							if aver > operand {
								fmt.Printf("[Detected] Aver: %f >  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f >  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
							}
						case operator == "<=":
							if aver <= operand {
								fmt.Printf("[Detected] Aver: %f <=  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f <=  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
							}
						case operator == "<":
							if aver < operand {
								fmt.Printf("[Detected] Aver: %f <  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f <  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
							}
						default:
							log.Debug().Msg("[Checking] Not available operator " + operator)
							mciPolicyTmp.Policy[policyIndex].Status = AutoStatusError
						}
					}
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case mciPolicyTmp.Policy[policyIndex].Status == AutoStatusChecking:
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					//mciPolicyTmp.Policy[policyIndex].Status = AutoStatusDetected

				case mciPolicyTmp.Policy[policyIndex].Status == AutoStatusDetected:
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					mciPolicyTmp.Policy[policyIndex].Status = AutoStatusOperating
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

					//Action
					/*
						// Actions for mci automation
						const AutoActionScaleOut string = "ScaleOut"
						const AutoActionScaleIn string = "ScaleIn"
					*/

					autoAction := mciPolicyTmp.Policy[policyIndex].AutoAction
					log.Debug().Msg("[autoAction] " + autoAction.ActionType)

					switch {
					case autoAction.ActionType == AutoActionScaleOut:

						autoAction.VmDynamicReq.Label = labelAutoGen
						// append UUID to given vm name to avoid duplicated vm ID.
						autoAction.VmDynamicReq.Name = common.ToLower(autoAction.VmDynamicReq.Name) + "-" + common.GenUid()
						//vmReqTmp := autoAction.Vm
						// autoAction.VmDynamicReq.SubGroupSize = "1"

						if autoAction.PlacementAlgo == "random" {
							log.Debug().Msg("[autoAction.PlacementAlgo] " + autoAction.PlacementAlgo)
							// var vmTmpErr error
							// existingVm, vmTmpErr := GetVmTemplate(nsId, mciPolicyTmp.Id, autoAction.PlacementAlgo)
							// if vmTmpErr != nil {
							// 	mciPolicyTmp.Policy[policyIndex].Status = AutoStatusError
							// 	UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							// }

							autoAction.VmDynamicReq.CommonImage = "ubuntu18.04"                // temporal default value. will be changed
							autoAction.VmDynamicReq.CommonSpec = "aws-ap-northeast-2-t2-small" // temporal default value. will be changed

							deploymentPlan := DeploymentPlan{}

							deploymentPlan.Priority.Policy = append(deploymentPlan.Priority.Policy, PriorityCondition{Metric: "random"})
							specList, err := RecommendVm(common.SystemCommonNs, deploymentPlan)
							if err != nil {
								mciPolicyTmp.Policy[policyIndex].Status = AutoStatusError
								UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							}
							if len(specList) != 0 {
								recommendedSpec := specList[0].Id
								autoAction.VmDynamicReq.CommonSpec = recommendedSpec
							}

							// autoAction.VmDynamicReq.Name = autoAction.VmDynamicReq.Name + "-random"
							// autoAction.VmDynamicReq.Label = labelAutoGen
						}

						common.PrintJsonPretty(autoAction.VmDynamicReq)
						log.Debug().Msg("[Action] " + autoAction.ActionType)

						// ScaleOut MCI according to the VM requirement.
						log.Debug().Msg("[Generating VM]")
						result, vmCreateErr := CreateMciVmDynamic(nsId, mciPolicyTmp.Id, &autoAction.VmDynamicReq)
						if vmCreateErr != nil {
							mciPolicyTmp.Policy[policyIndex].Status = AutoStatusError
							UpdateMciPolicyInfo(nsId, mciPolicyTmp)
						}
						common.PrintJsonPretty(result)

						if len(autoAction.PostCommand.Command) != 0 {

							log.Debug().Msgf("[Post Command to VM] %v", autoAction.PostCommand.Command)
							_, cmdErr := RemoteCommandToMci(nsId, mciPolicyTmp.Id, common.ToLower(autoAction.VmDynamicReq.Name), "", &autoAction.PostCommand)
							if cmdErr != nil {
								mciPolicyTmp.Policy[policyIndex].Status = AutoStatusError
								UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							}
						}

					case autoAction.ActionType == AutoActionScaleIn:
						log.Debug().Msg("[Action] " + autoAction.ActionType)

						// ScaleIn MCI.
						log.Debug().Msg("[Removing VM]")
						vmList, vmListErr := ListVmByLabel(nsId, mciPolicyTmp.Id, labelAutoGen)
						if vmListErr != nil {
							mciPolicyTmp.Policy[policyIndex].Status = AutoStatusError
							UpdateMciPolicyInfo(nsId, mciPolicyTmp)
						}
						if len(vmList) != 0 {
							removeTargetVm := vmList[len(vmList)-1]
							log.Debug().Msg("[Removing VM ID] " + removeTargetVm)
							delVmErr := DelMciVm(nsId, mciPolicyTmp.Id, removeTargetVm, "")
							if delVmErr != nil {
								mciPolicyTmp.Policy[policyIndex].Status = AutoStatusError
								UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							}
						}

					default:
					}

					mciPolicyTmp.Policy[policyIndex].Status = AutoStatusStabilizing
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case mciPolicyTmp.Policy[policyIndex].Status == AutoStatusStabilizing:
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

					//initialize Evaluation history so that controller does not act too early.
					//with this we can stablize MCI by init previously measures.
					//Will invoke [Checking] Not enough evaluationPeriod
					mciPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationValue = nil

					mciPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)

				case mciPolicyTmp.Policy[policyIndex].Status == AutoStatusOperating:
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					//mciPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
					//UpdateMciPolicyInfo(nsId, mciPolicyTmp)

				case mciPolicyTmp.Policy[policyIndex].Status == AutoStatusTimeout:
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case mciPolicyTmp.Policy[policyIndex].Status == AutoStatusError:
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					mciPolicyTmp.Policy[policyIndex].Status = AutoStatusReady
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)

				case mciPolicyTmp.Policy[policyIndex].Status == AutoStatusSuspended:
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				default:
				}
			}

		}

	}

}

// UpdateMciPolicyInfo updates MciPolicyInfo object in DB.
func UpdateMciPolicyInfo(nsId string, mciPolicyInfoData MciPolicyInfo) {
	key := common.GenMciPolicyKey(nsId, mciPolicyInfoData.Id, "")
	val, _ := json.Marshal(mciPolicyInfoData)
	err := kvstore.Put(key, string(val))
	if err != nil && !strings.Contains(err.Error(), common.ErrStrKeyNotFound) {
		log.Error().Err(err).Msg("")
	}
}

// CreateMciPolicy create MciPolicyInfo object in DB according to user's requirements.
func CreateMciPolicy(nsId string, mciId string, u *MciPolicyReq) (MciPolicyInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := MciPolicyInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := MciPolicyInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckMciPolicy(nsId, mciId)

	//u.Status = AutoStatusReady

	if check {
		temp := MciPolicyInfo{}
		err := fmt.Errorf("The MCI Policy Obj " + mciId + " already exists.")
		return temp, err
	}

	for policyIndex := range u.Policy {
		u.Policy[policyIndex].Status = AutoStatusReady
	}

	req := *u
	obj := MciPolicyInfo{}
	obj.Name = mciId
	obj.Id = mciId
	obj.Policy = req.Policy
	obj.Description = req.Description

	// kvstore
	Key := common.GenMciPolicyKey(nsId, obj.Id, "")
	Val, _ := json.Marshal(obj)

	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return obj, err
	}
	keyValue, err := kvstore.GetKv(Key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CreateMciPolicy(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	log.Debug().Msg("<KEY>\n" + keyValue.Key + "\n<VAL>\n" + keyValue.Value)

	return obj, nil
}

// GetMciPolicyObject returns MciPolicyInfo object.
func GetMciPolicyObject(nsId string, mciId string) (MciPolicyInfo, error) {
	log.Debug().Msg("[GetMciPolicyObject]" + mciId)

	key := common.GenMciPolicyKey(nsId, mciId, "")
	log.Debug().Msgf("Key: %v", key)
	keyValue, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return MciPolicyInfo{}, err
	}
	if keyValue == (kvstore.KeyValue{}) {
		return MciPolicyInfo{}, err
	}

	log.Debug().Msg("<KEY>\n" + keyValue.Key + "\n<VAL>\n" + keyValue.Value)

	mciPolicyTmp := MciPolicyInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mciPolicyTmp)
	return mciPolicyTmp, nil
}

// GetAllMciPolicyObject returns all MciPolicyInfo objects.
func GetAllMciPolicyObject(nsId string) ([]MciPolicyInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	Mci := []MciPolicyInfo{}
	mciList := ListMciPolicyId(nsId)

	for _, v := range mciList {

		key := common.GenMciPolicyKey(nsId, v, "")
		keyValue, err := kvstore.GetKv(key)
		if err != nil {
			log.Error().Err(err).Msg("")
			err = fmt.Errorf("In GetAllMciPolicyObject(); kvstore.GetKv() returned an error.")
			log.Error().Err(err).Msg("")
			// return nil, err
		}

		if keyValue == (kvstore.KeyValue{}) {
			return nil, fmt.Errorf("Cannot find " + key)
		}
		mciTmp := MciPolicyInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mciTmp)
		Mci = append(Mci, mciTmp)
	}

	return Mci, nil
}

// ListMciPolicyId returns a list of Ids for all MciPolicyInfo objects .
func ListMciPolicyId(nsId string) []string {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil
	}

	key := "/ns/" + nsId + "/policy/mci"
	keyValue, err := kvstore.GetKvList(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In ListMciPolicyId(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	var mciList []string
	for _, v := range keyValue {
		if !strings.Contains(v.Key, "vm") {
			mciList = append(mciList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/policy/mci/"))
		}
	}
	return mciList
}

// DelMciPolicy deletes MciPolicyInfo object by mciId.
func DelMciPolicy(nsId string, mciId string) error {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(mciId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	check, _ := CheckMciPolicy(nsId, mciId)

	if !check {
		err := fmt.Errorf("The mci Policy " + mciId + " does not exist.")
		return err
	}

	log.Debug().Msg("[Delete MCI Policy] " + mciId)

	key := common.GenMciPolicyKey(nsId, mciId, "")
	log.Debug().Msg(key)

	// delete mci Policy info
	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

// DelAllMciPolicy deletes all MciPolicyInfo objects.
func DelAllMciPolicy(nsId string) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	mciList := ListMciPolicyId(nsId)
	if len(mciList) == 0 {
		return "No MCI Policy to delete", nil
	}
	for _, v := range mciList {
		err := DelMciPolicy(nsId, v)
		if err != nil {
			log.Error().Err(err).Msg("")

			return "", fmt.Errorf("Failed to delete All MCI Policies")
		}
	}
	return "All MCI Policies has been deleted", nil
}
