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

// Package infra is to manage multi-cloud infra
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

// OrchestrationController is responsible for executing Infra automation policy.
// OrchestrationController will be periodically involked by a time.NewTicker in main.go.
func OrchestrationController() {

	nsList, err := common.ListNsId()
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("an error occurred while getting namespaces' list: " + err.Error())
		return
	}

	for _, nsId := range nsList {

		infraPolicyList := ListInfraPolicyId(nsId)

		for _, m := range infraPolicyList {
			log.Debug().Msg("infraPolicyList: NS[" + nsId + "]" + "InfraPolicy[" + m + "]")
		}

		for _, v := range infraPolicyList {

			key := common.GenInfraPolicyKey(nsId, v, "")
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
			infraPolicyTmp := model.InfraPolicyInfo{}
			json.Unmarshal([]byte(keyValue.Value), &infraPolicyTmp)

			/* FYI
			const model.AutoStatusReady string = "Ready"
			const model.AutoStatusChecking string = "Checking"
			const model.AutoStatusHappened string = "Happened"
			const model.AutoStatusOperating string = "Operating"
			const model.AutoStatusTimeout string = "Timeout"
			const model.AutoStatusError string = "Error"
			const model.AutoStatusSuspend string = "Suspend"
			*/

			for policyIndex := range infraPolicyTmp.Policy {
				log.Debug().Msg("\n[Infra-Policy-StateMachine] infraPolicyTmp.Policy[policyIndex],[" + v + "]")

				switch {
				case strings.EqualFold(infraPolicyTmp.Policy[policyIndex].Status, model.AutoStatusReady):
					log.Debug().Msg("- PolicyStatus[" + model.AutoStatusReady + "],[" + v + "]")
					infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusChecking
					UpdateInfraPolicyInfo(nsId, infraPolicyTmp)

					log.Debug().Msg("[Check Infra Policy] " + infraPolicyTmp.Id)
					check, err := CheckInfra(nsId, infraPolicyTmp.Id)
					log.Debug().Msg("[Check existence of Infra] " + infraPolicyTmp.Id)
					//keyValueInfra, _ := common.Ckvstore.GetKv(common.GenInfraKey(nsId, infraPolicyTmp.Id, ""))

					if !check || err != nil {
						infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
						UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
						log.Debug().Msg("[Infra is not exist] " + infraPolicyTmp.Id)
						break
					} else { // need to enhance : loop for each policies and realize metric

						//Checking (measuring)
						infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusChecking
						UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
						log.Debug().Msg("- PolicyStatus[" + infraPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

						log.Debug().Msg("[Infra is exist] " + infraPolicyTmp.Id)
						content, err := GetMonitoringData(nsId, infraPolicyTmp.Id, infraPolicyTmp.Policy[policyIndex].AutoCondition.Metric)
						if err != nil {
							log.Error().Err(err).Msg("")
							infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
							break
						}

						//Statistic
						sumInfra := 0.0
						for _, monData := range content.InfraMonitoring {
							monDataValue, _ := strconv.ParseFloat(monData.Value, 64)
							sumInfra += monDataValue
						}
						averInfra := (sumInfra / float64(len(content.InfraMonitoring)))
						fmt.Printf("[monData.Value] AverInfra: %f,  SumInfra: %f \n", averInfra, sumInfra)

						// Before adding new value, ensure the list doesn't exceed evaluationPeriod length
						evaluationPeriod := infraPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationPeriod
						evaluationValue := infraPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationValue

						// Add new value to the beginning of the list
						evaluationValue = append([]string{fmt.Sprintf("%f", averInfra)}, evaluationValue...)

						// If the list exceeds evaluationPeriod, truncate it
						if len(evaluationValue) > evaluationPeriod {
							// Keep only the most recent evaluationPeriod items
							evaluationValue = evaluationValue[:evaluationPeriod]
						}

						// Update the evaluationValue in the policy
						infraPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationValue = evaluationValue

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
						operator := infraPolicyTmp.Policy[policyIndex].AutoCondition.Operator
						operand := infraPolicyTmp.Policy[policyIndex].AutoCondition.Operand

						if evaluationPeriod == 0 {
							log.Debug().Msg("[Checking] Not available evaluationPeriod ")
							infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
							UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
							break
						}
						// not enough evaluationPeriod
						if aver == -0.1 {
							log.Debug().Msg("[Checking] Not enough evaluationPeriod ")
							infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
							UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
							break
						}
						switch {
						case operator == ">=":
							if aver >= operand {
								fmt.Printf("[Detected] Aver: %f >=  Operand: %f \n", aver, operand)
								infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusDetected
							} else {
								fmt.Printf("[Not Detected] Aver: %f >=  Operand: %f \n", aver, operand)
								infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
							}
						case operator == ">":
							if aver > operand {
								fmt.Printf("[Detected] Aver: %f >  Operand: %f \n", aver, operand)
								infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f >  Operand: %f \n", aver, operand)
								infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
							}
						case operator == "<=":
							if aver <= operand {
								fmt.Printf("[Detected] Aver: %f <=  Operand: %f \n", aver, operand)
								infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f <=  Operand: %f \n", aver, operand)
								infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
							}
						case operator == "<":
							if aver < operand {
								fmt.Printf("[Detected] Aver: %f <  Operand: %f \n", aver, operand)
								infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusDetected

							} else {
								fmt.Printf("[Not Detected] Aver: %f <  Operand: %f \n", aver, operand)
								infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
							}
						default:
							log.Debug().Msg("[Checking] Not available operator " + operator)
							infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
						}
					}
					UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
					log.Debug().Msg("- PolicyStatus[" + infraPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case strings.EqualFold(infraPolicyTmp.Policy[policyIndex].Status, model.AutoStatusChecking):
					log.Debug().Msg("- PolicyStatus[" + infraPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					//infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusDetected

				case strings.EqualFold(infraPolicyTmp.Policy[policyIndex].Status, model.AutoStatusDetected):
					log.Debug().Msg("- PolicyStatus[" + infraPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusOperating
					UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
					log.Debug().Msg("- PolicyStatus[" + infraPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

					//Action
					/*
						// Actions for infra automation
						const model.AutoActionScaleOut string = "ScaleOut"
						const model.AutoActionScaleIn string = "ScaleIn"
					*/

					autoAction := infraPolicyTmp.Policy[policyIndex].AutoAction
					log.Debug().Msg("[autoAction] " + autoAction.ActionType)

					switch {
					case strings.EqualFold(autoAction.ActionType, model.AutoActionScaleOut):

						labels := map[string]string{
							model.LabelDeploymentType: model.StrAutoGen,
						}
						autoAction.NodeGroupDynamicReq.Label = labels
						// append uid to given vm name to avoid duplicated vm ID.
						autoAction.NodeGroupDynamicReq.Name = common.ToLower(autoAction.NodeGroupDynamicReq.Name) + "-" + common.GenUid()
						//vmReqTmp := autoAction.Vm
						// autoAction.NodeGroupDynamicReq.NodeGroupSize = "1"

						if strings.EqualFold(autoAction.PlacementAlgo, "random") {
							log.Debug().Msg("[autoAction.PlacementAlgo] " + autoAction.PlacementAlgo)
							// var vmTmpErr error
							// existingVm, vmTmpErr := GetVmTemplate(nsId, infraPolicyTmp.Id, autoAction.PlacementAlgo)
							// if vmTmpErr != nil {
							// 	infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
							// 	UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
							// }

							autoAction.NodeGroupDynamicReq.ImageId = "ubuntu18.04"                // temporal default value. will be changed
							autoAction.NodeGroupDynamicReq.SpecId = "aws-ap-northeast-2-t2-small" // temporal default value. will be changed

							recommendSpecReq := model.RecommendSpecReq{}

							recommendSpecReq.Priority.Policy = append(recommendSpecReq.Priority.Policy, model.PriorityCondition{Metric: "random"})
							specList, err := RecommendSpec(common.NewDefaultContext(), model.SystemCommonNs, recommendSpecReq)
							if err != nil {
								infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
								UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
							}
							if len(specList) != 0 {
								recommendedSpec := specList[0].Id
								autoAction.NodeGroupDynamicReq.SpecId = recommendedSpec
							}

							// autoAction.NodeGroupDynamicReq.Name = autoAction.NodeGroupDynamicReq.Name + "-random"
							// autoAction.NodeGroupDynamicReq.Label = model.LabelAutoGen
						}

						common.PrintJsonPretty(autoAction.NodeGroupDynamicReq)
						log.Debug().Msg("[Action] " + autoAction.ActionType)

						// ScaleOut Infra according to the VM requirement.
						log.Debug().Msg("[Generating VM]")
						result, vmCreateErr := CreateInfraNodeGroupDynamic(common.NewDefaultContext(), nsId, infraPolicyTmp.Id, &autoAction.NodeGroupDynamicReq)
						if vmCreateErr != nil {
							infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
							UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
						}
						common.PrintJsonPretty(result)

						if len(autoAction.PostCommand.Command) != 0 {

							log.Debug().Msgf("[Post Command to VM] %v", autoAction.PostCommand.Command)
							_, cmdErr := RemoteCommandToInfra(nsId, infraPolicyTmp.Id, common.ToLower(autoAction.NodeGroupDynamicReq.Name), "", "", &autoAction.PostCommand, "")
							if cmdErr != nil {
								infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
								UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
							}
						}

					case strings.EqualFold(autoAction.ActionType, model.AutoActionScaleIn):
						log.Debug().Msg("[Action] " + autoAction.ActionType)

						// ScaleIn Infra.
						log.Debug().Msg("[Removing VM]")
						vmList, vmListErr := ListVmByLabel(nsId, infraPolicyTmp.Id, model.StrAutoGen)
						if vmListErr != nil {
							infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
							UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
						}
						if len(vmList) != 0 {
							removeTargetVm := vmList[len(vmList)-1]
							log.Debug().Msg("[Removing VM ID] " + removeTargetVm)
							delVmErr := DelInfraVm(nsId, infraPolicyTmp.Id, removeTargetVm, "")
							if delVmErr != nil {
								infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusError
								UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
							}
						}

					default:
					}

					infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusStabilizing
					UpdateInfraPolicyInfo(nsId, infraPolicyTmp)
					log.Debug().Msg("- PolicyStatus[" + infraPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case strings.EqualFold(infraPolicyTmp.Policy[policyIndex].Status, model.AutoStatusStabilizing):
					log.Debug().Msg("- PolicyStatus[" + infraPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

					//initialize Evaluation history so that controller does not act too early.
					//with this we can stablize Infra by init previously measures.
					//Will invoke [Checking] Not enough evaluationPeriod
					infraPolicyTmp.Policy[policyIndex].AutoCondition.EvaluationValue = nil

					infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
					UpdateInfraPolicyInfo(nsId, infraPolicyTmp)

				case strings.EqualFold(infraPolicyTmp.Policy[policyIndex].Status, model.AutoStatusOperating):
					log.Debug().Msg("- PolicyStatus[" + infraPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					//infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
					//UpdateInfraPolicyInfo(nsId, infraPolicyTmp)

				case strings.EqualFold(infraPolicyTmp.Policy[policyIndex].Status, model.AutoStatusTimeout):
					log.Debug().Msg("- PolicyStatus[" + infraPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				case strings.EqualFold(infraPolicyTmp.Policy[policyIndex].Status, model.AutoStatusError):
					log.Debug().Msg("- PolicyStatus[" + infraPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")
					infraPolicyTmp.Policy[policyIndex].Status = model.AutoStatusReady
					UpdateInfraPolicyInfo(nsId, infraPolicyTmp)

				case strings.EqualFold(infraPolicyTmp.Policy[policyIndex].Status, model.AutoStatusSuspended):
					log.Debug().Msg("- PolicyStatus[" + infraPolicyTmp.Policy[policyIndex].Status + "],[" + v + "]")

				default:
				}
			}

		}

	}

}

// UpdateInfraPolicyInfo updates model.InfraPolicyInfo object in DB.
func UpdateInfraPolicyInfo(nsId string, infraPolicyInfoData model.InfraPolicyInfo) {
	key := common.GenInfraPolicyKey(nsId, infraPolicyInfoData.Id, "")
	val, _ := json.Marshal(infraPolicyInfoData)
	err := kvstore.Put(key, string(val))
	if err != nil && !strings.Contains(err.Error(), model.ErrStrKeyNotFound) {
		log.Error().Err(err).Msg("")
	}
}

// CreateInfraPolicy create model.InfraPolicyInfo object in DB according to user's requirements.
func CreateInfraPolicy(nsId string, infraId string, u *model.InfraPolicyReq) (model.InfraPolicyInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		temp := model.InfraPolicyInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}

	err = common.CheckString(infraId)
	if err != nil {
		temp := model.InfraPolicyInfo{}
		log.Error().Err(err).Msg("")
		return temp, err
	}
	check, _ := CheckInfraPolicy(nsId, infraId)

	//u.Status = model.AutoStatusReady

	if check {
		temp := model.InfraPolicyInfo{}
		err := fmt.Errorf("The Infra Policy Obj " + infraId + " already exists.")
		return temp, err
	}

	for policyIndex := range u.Policy {
		u.Policy[policyIndex].Status = model.AutoStatusReady
	}

	req := *u
	obj := model.InfraPolicyInfo{}
	obj.Name = infraId
	obj.Id = infraId
	obj.Policy = req.Policy
	obj.Description = req.Description

	// kvstore
	Key := common.GenInfraPolicyKey(nsId, obj.Id, "")
	Val, _ := json.Marshal(obj)

	err = kvstore.Put(Key, string(Val))
	if err != nil {
		log.Error().Err(err).Msg("")
		return obj, err
	}
	keyValue, _, err := kvstore.GetKv(Key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In CreateInfraPolicy(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	log.Debug().Msg("<KEY>\n" + keyValue.Key + "\n<VAL>\n" + keyValue.Value)

	return obj, nil
}

// GetInfraPolicyObject returns model.InfraPolicyInfo object.
func GetInfraPolicyObject(nsId string, infraId string) (model.InfraPolicyInfo, error) {
	log.Debug().Msg("[GetInfraPolicyObject]" + infraId)

	key := common.GenInfraPolicyKey(nsId, infraId, "")
	log.Debug().Msgf("Key: %v", key)
	keyValue, exists, err := kvstore.GetKv(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return model.InfraPolicyInfo{}, err
	}
	if !exists {
		return model.InfraPolicyInfo{}, err
	}

	log.Debug().Msg("<KEY>\n" + keyValue.Key + "\n<VAL>\n" + keyValue.Value)

	infraPolicyTmp := model.InfraPolicyInfo{}
	json.Unmarshal([]byte(keyValue.Value), &infraPolicyTmp)
	return infraPolicyTmp, nil
}

// GetAllInfraPolicyObject returns all model.InfraPolicyInfo objects.
func GetAllInfraPolicyObject(nsId string) ([]model.InfraPolicyInfo, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil, err
	}
	Infra := []model.InfraPolicyInfo{}
	infraList := ListInfraPolicyId(nsId)

	for _, v := range infraList {

		key := common.GenInfraPolicyKey(nsId, v, "")
		keyValue, exists, err := kvstore.GetKv(key)
		if err != nil {
			log.Error().Err(err).Msg("")
			err = fmt.Errorf("In GetAllInfraPolicyObject(); kvstore.GetKv() returned an error.")
			log.Error().Err(err).Msg("")
			// return nil, err
		}

		if !exists {
			return nil, fmt.Errorf("Cannot find " + key)
		}
		infraTmp := model.InfraPolicyInfo{}
		json.Unmarshal([]byte(keyValue.Value), &infraTmp)
		Infra = append(Infra, infraTmp)
	}

	return Infra, nil
}

// ListInfraPolicyId returns a list of Ids for all model.InfraPolicyInfo objects .
func ListInfraPolicyId(nsId string) []string {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return nil
	}

	key := "/ns/" + nsId + "/policy/infra"
	keyValue, err := kvstore.GetKvList(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		err = fmt.Errorf("In ListInfraPolicyId(); kvstore.GetKv() returned an error.")
		log.Error().Err(err).Msg("")
		// return nil, err
	}

	var infraList []string
	for _, v := range keyValue {
		if !strings.Contains(v.Key, "vm") {
			infraList = append(infraList, strings.TrimPrefix(v.Key, "/ns/"+nsId+"/policy/infra/"))
		}
	}
	return infraList
}

// DelInfraPolicy deletes model.InfraPolicyInfo object by infraId.
func DelInfraPolicy(nsId string, infraId string) error {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	err = common.CheckString(infraId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}
	check, _ := CheckInfraPolicy(nsId, infraId)

	if !check {
		err := fmt.Errorf("The infra Policy " + infraId + " does not exist.")
		return err
	}

	log.Debug().Msg("[Delete Infra Policy] " + infraId)

	key := common.GenInfraPolicyKey(nsId, infraId, "")
	log.Debug().Msg(key)

	// delete infra Policy info
	err = kvstore.Delete(key)
	if err != nil {
		log.Error().Err(err).Msg("")
		return err
	}

	return nil
}

// DelAllInfraPolicy deletes all model.InfraPolicyInfo objects.
func DelAllInfraPolicy(nsId string) (string, error) {

	err := common.CheckString(nsId)
	if err != nil {
		log.Error().Err(err).Msg("")
		return "", err
	}
	infraList := ListInfraPolicyId(nsId)
	if len(infraList) == 0 {
		return "No Infra Policy to delete", nil
	}
	for _, v := range infraList {
		err := DelInfraPolicy(nsId, v)
		if err != nil {
			log.Error().Err(err).Msg("")

			return "", fmt.Errorf("Failed to delete All Infra Policies")
		}
	}
	return "All Infra Policies has been deleted", nil
}
