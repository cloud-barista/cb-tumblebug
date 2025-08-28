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

// Package mci is to manage multi-cloud infra
package infra

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/rs/zerolog/log"
)

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
			log.Debug().Msg("mciPolicyList: NS[" + nsId + "]" + "MciPolicy[" + m + "]")
		}

		for _, v := range mciPolicyList {

			key := common.GenMciPolicyKey(nsId, v, "")
			keyValue, exists, err := kvstore.GetKv(key)
			if err != nil {
				log.Error().Err(err).Msg("")
				err = fmt.Errorf("In OrchestrationController(); kvstore.GetKv() returned an error.")
				log.Error().Err(err).Msg("")
				// return nil, err
			}

			if !exists {
				log.Debug().Msg("keyValue is nil")
			}
			mciPolicyTmp := model.MciPolicyInfo{}
			json.Unmarshal([]byte(keyValue.Value), &mciPolicyTmp)

			/* FYI
			const model.AutoStatusReady string = "Ready"
			const model.AutoStatusChecking string = "Checking"
			const model.AutoStatusHappened string = "Happened"
			const model.AutoStatusOperating string = "Operating"
			const model.AutoStatusTimeout string = "Timeout"
			const model.AutoStatusError string = "Error"
			const model.AutoStatusSuspend string = "Suspend"
			*/

			for policyIndex := range mciPolicyTmp.Policy {
				log.Debug().Msg("\n[MCI-Policy-StateMachine] mciPolicyTmp.Policy[policyIndex],[" + v + "]")

				switch {
				case strings.EqualFold(mciPolicyTmp.Policy[policyIndex].Status, model.AutoStatusReady):
					log.Debug().Msg("- PolicyStatus[" + model.AutoStatusReady + "],[" + v + "]")
					mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusChecking
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)

					log.Debug().Msg("[Check MCI Policy] " + mciPolicyTmp.Id)
					check, err := CheckMci(nsId, mciPolicyTmp.Id)
					log.Debug().Msg("[Check existence of MCI] " + mciPolicyTmp.Id)
					//keyValueMci, _ := common.Ckvstore.GetKv(common.GenMciKey(nsId, mciPolicyTmp.Id, ""))

					if !check || err != nil {
						mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
						UpdateMciPolicyInfo(nsId, mciPolicyTmp)
						log.Debug().Msg("[MCI is not exist] " + mciPolicyTmp.Id)
						break
					} else { // need to enhance : loop for each policies and realize metric

						//Checking (measuring)
						mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusChecking
						UpdateMciPolicyInfo(nsId, mciPolicyTmp)
						log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

						log.Debug().Msg("[MCI is exist] " + mciPolicyTmp.Id)
						content, err := GetMonitoringData(nsId, mciPolicyTmp.Id, mciPolicyTmp.Policy[policyIndex].AutoCondition.Metric)
						if err != nil {
							log.Error().Err(err).Msg("")
							mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
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

						// Before adding new value, ensure the list doesn't exceed evaluationPeriod length
						evaluationPeriod, _ := strconv.Atoi(mciPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationPeriod)
						evaluationValue := mciPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationValue

						// Add new value to the beginning of the list
						evaluationValue = append([]string{fmt.Sprintf("%f", averMci)}, evaluationValue...)

						// If the list exceeds evaluationPeriod, truncate it
						if len(evaluationValue) > evaluationPeriod {
							// Keep only the most recent evaluationPeriod items
							evaluationValue = evaluationValue[:evaluationPeriod]
						}

						// Update the evaluationValue in the policy
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
							mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
							UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							break
						}
						// not enough evaluationPeriod
						if aver == -0.1 {
							log.Debug().Msg("[Checking] Not enough evaluationPeriod ")
							mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
							UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							break
						}
						switch {
						case operator == ">=":
							if aver >= operand {
								fmt.Printf("[Detected] Aver: %f >=  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusDetected
							} else {
								fmt.Printf("[Not Detected] Aver: %f >=  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
							}
						case operator == ">":
							if aver > operand {
								fmt.Printf("[Detected] Aver: %f >  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f >  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
							}
						case operator == "<=":
							if aver <= operand {
								fmt.Printf("[Detected] Aver: %f <=  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f <=  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
							}
						case operator == "<":
							if aver < operand {
								fmt.Printf("[Detected] Aver: %f <  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f <  Operand: %f \n", aver, operand)
								mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
							}
						default:
							log.Debug().Msg("[Checking] Not available operator " + operator)
							mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
						}
					}
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case strings.EqualFold(mciPolicyTmp.Policy[policyIndex].Status, model.AutoStatusChecking):
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					//mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusDetected

				case strings.EqualFold(mciPolicyTmp.Policy[policyIndex].Status, model.AutoStatusDetected):
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusOperating
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

					//Action
					/*
						// Actions for mci automation
						const model.AutoActionScaleOut string = "ScaleOut"
						const model.AutoActionScaleIn string = "ScaleIn"
					*/

					autoAction := mciPolicyTmp.Policy[policyIndex].AutoAction
					log.Debug().Msg("[autoAction] " + autoAction.ActionType)

					switch {
					case strings.EqualFold(autoAction.ActionType, model.AutoActionScaleOut):

						labels := map[string]string{
							model.LabelDeploymentType: model.StrAutoGen,
						}
						autoAction.SubGroupDynamicReq.Label = labels
						// append uid to given vm name to avoid duplicated vm ID.
						autoAction.SubGroupDynamicReq.Name = common.ToLower(autoAction.SubGroupDynamicReq.Name) + "-" + common.GenUid()
						//vmReqTmp := autoAction.Vm
						// autoAction.SubGroupDynamicReq.SubGroupSize = "1"

						if strings.EqualFold(autoAction.PlacementAlgo, "random") {
							log.Debug().Msg("[autoAction.PlacementAlgo] " + autoAction.PlacementAlgo)
							// var vmTmpErr error
							// existingVm, vmTmpErr := GetVmTemplate(nsId, mciPolicyTmp.Id, autoAction.PlacementAlgo)
							// if vmTmpErr != nil {
							// 	mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
							// 	UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							// }

							autoAction.SubGroupDynamicReq.ImageId = "ubuntu18.04"                // temporal default value. will be changed
							autoAction.SubGroupDynamicReq.SpecId = "aws-ap-northeast-2-t2-small" // temporal default value. will be changed

							recommendSpecReq := model.RecommendSpecReq{}

							recommendSpecReq.Priority.Policy = append(recommendSpecReq.Priority.Policy, model.PriorityCondition{Metric: "random"})
							specList, err := RecommendSpec(model.SystemCommonNs, recommendSpecReq)
							if err != nil {
								mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
								UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							}
							if len(specList) != 0 {
								recommendedSpec := specList[0].Id
								autoAction.SubGroupDynamicReq.SpecId = recommendedSpec
							}

							// autoAction.SubGroupDynamicReq.Name = autoAction.SubGroupDynamicReq.Name + "-random"
							// autoAction.SubGroupDynamicReq.Label = model.LabelAutoGen
						}

						common.PrintJsonPretty(autoAction.SubGroupDynamicReq)
						log.Debug().Msg("[Action] " + autoAction.ActionType)

						// ScaleOut MCI according to the VM requirement.
						log.Debug().Msg("[Generating VM]")
						result, vmCreateErr := CreateMciSubGroupDynamic(nsId, mciPolicyTmp.Id, &autoAction.SubGroupDynamicReq)
						if vmCreateErr != nil {
							mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
							UpdateMciPolicyInfo(nsId, mciPolicyTmp)
						}
						common.PrintJsonPretty(result)

						if len(autoAction.PostCommand.Command) != 0 {

							log.Debug().Msgf("[Post Command to VM] %v", autoAction.PostCommand.Command)
							_, cmdErr := RemoteCommandToMci(nsId, mciPolicyTmp.Id, common.ToLower(autoAction.SubGroupDynamicReq.Name), "", "", &autoAction.PostCommand)
							if cmdErr != nil {
								mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
								UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							}
						}

					case strings.EqualFold(autoAction.ActionType, model.AutoActionScaleIn):
						log.Debug().Msg("[Action] " + autoAction.ActionType)

						// ScaleIn MCI.
						log.Debug().Msg("[Removing VM]")
						vmList, vmListErr := ListVmByLabel(nsId, mciPolicyTmp.Id, model.StrAutoGen)
						if vmListErr != nil {
							mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
							UpdateMciPolicyInfo(nsId, mciPolicyTmp)
						}
						if len(vmList) != 0 {
							removeTargetVm := vmList[len(vmList)-1]
							log.Debug().Msg("[Removing VM ID] " + removeTargetVm)
							delVmErr := DelMciVm(nsId, mciPolicyTmp.Id, removeTargetVm, "")
							if delVmErr != nil {
								mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
								UpdateMciPolicyInfo(nsId, mciPolicyTmp)
							}
						}

					default:
					}

					mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusStabilizing
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case strings.EqualFold(mciPolicyTmp.Policy[policyIndex].Status, model.AutoStatusStabilizing):
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

					//initialize Evaluation history so that controller does not act too early.
					//with this we can stablize MCI by init previously measures.
					//Will invoke [Checking] Not enough evaluationPeriod
					mciPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationValue = nil

					mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)

				case strings.EqualFold(mciPolicyTmp.Policy[policyIndex].Status, model.AutoStatusOperating):
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					//mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
					//UpdateMciPolicyInfo(nsId, mciPolicyTmp)

				case strings.EqualFold(mciPolicyTmp.Policy[policyIndex].Status, model.AutoStatusTimeout):
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case strings.EqualFold(mciPolicyTmp.Policy[policyIndex].Status, model.AutoStatusError):
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					mciPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
					UpdateMciPolicyInfo(nsId, mciPolicyTmp)

				case strings.EqualFold(mciPolicyTmp.Policy[policyIndex].Status, model.AutoStatusSuspended):
					log.Debug().Msg("- PolicyStatus[" + mciPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				default:
				}
			}

		}

	}

}

// UpdateMciPolicyInfo updates model.MciPolicyInfo object in DB.
func UpdateMciPolicyInfo(nsId string, mciPolicyInfoData model.MciPolicyInfo) {
	key := common.GenMciPolicyKey(nsId, mciPolicyInfoData.Id, "")
	val, _ := json.Marshal(mciPolicyInfoData)
	err := kvstore.Put(key, string(val))
	if err != nil && !strings.Contains(err.Error(), model.ErrStrKeyNotFound) {
		log.Error().Err(err).Msg("")
	}
}

// CreateMciPolicy create model.MciPolicyInfo object in DB according to user's requirements.
func CreateMciPolicy(nsId string, mciId string, u *model.MciPolicyReq) (model.MciPolicyInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.MciPolicyInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(mciId)
	if err != nil {
		temp := model.MciPolicyInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckMciPolicy(nsId, mciId)

	//u.Status = model.AutoStatusReady

	if check {
		temp := model.MciPolicyInfo{}
		err := fmt.Errorf("The MCI Policy Obj " + mciId + " already exists.")
		return temp, err
	}

	for policyIndex := range u.Policy {
		u.Policy[policyIndex].Status = model.AutoStatusReady
	}

	req := *u
	obj := model.MciPolicyInfo{}
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
	keyValue, _, err := kvstore.GetKv(Key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CreateMciPolicy(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	log.Debug().Msg("<KEY>\n" + keyValue.Key + "\n<VAL>\n" + keyValue.Value)

	return obj, nil
}

// GetMciPolicyObject returns model.MciPolicyInfo object.
func GetMciPolicyObject(nsId string, mciId string) (model.MciPolicyInfo, error) {
	log.Debug().Msg("[GetMciPolicyObject]" + mciId)

	key := common.GenMciPolicyKey(nsId, mciId, "")
	log.Debug().Msgf("Key: %v", key)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.MciPolicyInfo{}, err
	}
	if !exists {
		return model.MciPolicyInfo{}, err
	}

	log.Debug().Msg("<KEY>\n" + keyValue.Key + "\n<VAL>\n" + keyValue.Value)

	mciPolicyTmp := model.MciPolicyInfo{}
	json.Unmarshal([]byte(keyValue.Value), &mciPolicyTmp)
	return mciPolicyTmp, nil
}

// GetAllMciPolicyObject returns all model.MciPolicyInfo objects.
func GetAllMciPolicyObject(nsId string) ([]model.MciPolicyInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	Mci := []model.MciPolicyInfo{}
	mciList := ListMciPolicyId(nsId)

	for _, v := range mciList {

		key := common.GenMciPolicyKey(nsId, v, "")
		keyValue, exists, err := kvstore.GetKv(key)
		if err != nil {
			log.Error().Err(err).Msg("")
			err = fmt.Errorf("In GetAllMciPolicyObject(); kvstore.GetKv() returned an error.")
			log.Error().Err(err).Msg("")
			// return nil, err
		}

		if !exists {
			return nil, fmt.Errorf("Cannot find " + key)
		}
		mciTmp := model.MciPolicyInfo{}
		json.Unmarshal([]byte(keyValue.Value), &mciTmp)
		Mci = append(Mci, mciTmp)
	}

	return Mci, nil
}

// ListMciPolicyId returns a list of Ids for all model.MciPolicyInfo objects .
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

// DelMciPolicy deletes model.MciPolicyInfo object by mciId.
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

// DelAllMciPolicy deletes all model.MciPolicyInfo objects.
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
