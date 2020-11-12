package mcis

import (

	"encoding/json"
	"fmt"
	"strings"
	"strconv"


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
	Metric            string     `json:"metric"`
	Operator          string     `json:"operator"` // <, <=, >, >=, ...
	Operand           string     `json:"operand"`   // 10, 70, 80, 98, ...
	EvaluationPeriod  string     `json:"evaluationPeriod"`   // evaluationPeriod
	EvaluationValue   []string   `json:"evaluationValue"`
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

func OrchestrationController() {

	nsList := common.ListNsId()

	fmt.Println("")
	for _, nsId := range nsList {
		fmt.Println("NS[" + nsId + "]")
		mcisPolicyList := ListMcisPolicyId(nsId)

		fmt.Println("\n[MCIS Policy List]")
		for _, m := range mcisPolicyList {
			fmt.Println("McisPolicy[" + m + "]")
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
			fmt.Println("\n[MCIS-Policy-StateMachine]")
			switch {
				case mcisPolicyTmp.Status == AutoStatusReady:
					fmt.Println("- PolicyStatus[" + AutoStatusReady + "],["+ v + "]")
					mcisPolicyTmp.Status = AutoStatusChecking
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
 					
					fmt.Println("[Check MCIS Policy] " + mcisPolicyTmp.Id)
					check, _, _ := LowerizeAndCheckMcis(nsId, mcisPolicyTmp.Id ) 
					fmt.Println("[Check existance of MCIS] " + mcisPolicyTmp.Id)
					//keyValueMcis, _ := common.CBStore.Get(common.GenMcisKey(nsId, mcisPolicyTmp.Id, ""))
					
					if !check {
						mcisPolicyTmp.Status = AutoStatusError
						UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
						fmt.Println("[MCIS is not exist] " + mcisPolicyTmp.Id)
						break	
					} else { // need to enhance : loop for each policies and realize metric

						//Checking (measuring)
						mcisPolicyTmp.Status = AutoStatusChecking
						UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
						fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Status + "],["+ v + "]")

						fmt.Println("[MCIS is exist] " + mcisPolicyTmp.Id)
						content, err := GetMonitoringData(nsId, mcisPolicyTmp.Id, mcisPolicyTmp.Policy[0].AutoCondition.Metric)
						if err != nil {
							common.CBLog.Error(err)
							mcisPolicyTmp.Status = AutoStatusError
							break
						}
						//common.PrintJsonPretty(content)

						//Statistic
						sumMcis := 0.0
						for _, monData := range content.McisMonitoring {
							fmt.Println("[monData.Value: ] " + monData.Value)
							monDataValue, _ := strconv.ParseFloat(monData.Value, 64)
							sumMcis += monDataValue
						}
						averMcis := (sumMcis / float64(len(content.McisMonitoring)))
						fmt.Printf("[monData.Value] AverMcis: %f,  SumMcis: %f \n", averMcis, sumMcis)

						evaluationPeriod, _ := strconv.Atoi(mcisPolicyTmp.Policy[0].AutoCondition.EvaluationPeriod)
						evaluationValue := mcisPolicyTmp.Policy[0].AutoCondition.EvaluationValue 
						evaluationValue = append( []string{fmt.Sprintf("%f", averMcis)}, evaluationValue... ) // prepend current aver date
						mcisPolicyTmp.Policy[0].AutoCondition.EvaluationValue = evaluationValue

						sum := 0.0
						aver := -0.1
						// accumerate previous evaluation value
						fmt.Printf("[Evaluation History]\n")
						for evi, evv := range evaluationValue {
							evvFloat, _ := strconv.ParseFloat(evv, 64)
							sum += evvFloat
							fmt.Printf("[%v] %f ", evi, evvFloat)
							// break with outside evaluationValue
							if evi >= evaluationPeriod - 1 {
								break
							}
						}
						// average for evaluationPeriod (if data for the period is not enough, skip)
						if evaluationPeriod != 0 && len(evaluationValue) >= evaluationPeriod {
							aver = sum / float64(evaluationPeriod)
						}
						fmt.Printf("\n[Evaluation] Aver: %f,  Period: %v \n", aver, evaluationPeriod)
						
						//Detecting
						operator := mcisPolicyTmp.Policy[0].AutoCondition.Operator
						operand, _ := strconv.ParseFloat(mcisPolicyTmp.Policy[0].AutoCondition.Operand, 64)

						if evaluationPeriod == 0 {
							fmt.Println("[Checking] Not available evaluationPeriod ")
							mcisPolicyTmp.Status = AutoStatusError
							UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
							break
						}
						// not enough evaluationPeriod
						if aver == -0.1 {
							fmt.Println("[Checking] Not enough evaluationPeriod ")
							mcisPolicyTmp.Status = AutoStatusReady
							UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
							break
						}
						switch {
							case operator == ">=":
								if aver >= operand {
									fmt.Printf("[Detected] Aver: %f >=  Operand: %f \n", aver, operand)
									mcisPolicyTmp.Status = AutoStatusDetected
								} else {
									fmt.Printf("[Not Detected] Aver: %f >=  Operand: %f \n", aver, operand)
									mcisPolicyTmp.Status = AutoStatusReady
								}
							case operator == ">":
								if aver > operand {
									fmt.Printf("[Detected] Aver: %f >  Operand: %f \n", aver, operand)
									mcisPolicyTmp.Status = AutoStatusDetected
									
								} else {
									fmt.Printf("[Not Detected] Aver: %f >  Operand: %f \n", aver, operand)
									mcisPolicyTmp.Status = AutoStatusReady
								}
							case operator == "<=":
								if aver <= operand {
									fmt.Printf("[Detected] Aver: %f <=  Operand: %f \n", aver, operand)
									mcisPolicyTmp.Status = AutoStatusDetected
									
								} else {
									fmt.Printf("[Not Detected] Aver: %f <=  Operand: %f \n", aver, operand)
									mcisPolicyTmp.Status = AutoStatusReady
								}
							case operator == "<":
								if aver < operand {
									fmt.Printf("[Detected] Aver: %f <  Operand: %f \n", aver, operand)
									mcisPolicyTmp.Status = AutoStatusDetected
									
								} else {
									fmt.Printf("[Not Detected] Aver: %f <  Operand: %f \n", aver, operand)
									mcisPolicyTmp.Status = AutoStatusReady
								}
							default:
								fmt.Println("[Checking] Not available operator " + operator)
								mcisPolicyTmp.Status = AutoStatusError
						}
					}
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Status + "],["+ v + "]")


				case mcisPolicyTmp.Status == AutoStatusChecking:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Status + "],["+ v + "]")
					//mcisPolicyTmp.Status = AutoStatusDetected

				case mcisPolicyTmp.Status == AutoStatusDetected:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Status + "],["+ v + "]")
					mcisPolicyTmp.Status = AutoStatusOperating
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Status + "],["+ v + "]")

					//Action
					/*
					// Actions for mcis automation
					const AutoActionScaleOut string = "ScaleOut"
					const AutoActionScaleIn string = "ScaleIn"
					*/

					autoAction := mcisPolicyTmp.Policy[0].AutoAction
					fmt.Println("[autoAction] "+ autoAction.ActionType)

					switch {
						case autoAction.ActionType == AutoActionScaleOut:
							autoAction.Vm.Label = "AUTOGEN"
							common.PrintJsonPretty(autoAction.Vm)
							fmt.Println("[Action] "+ autoAction.ActionType)

							// Expand MCIS according to the VM requirement.
							result, vmCreateErr := CorePostMcisVm(nsId, mcisPolicyTmp.Id, &autoAction.Vm)
							if vmCreateErr != nil {
								mcisPolicyTmp.Status = AutoStatusError
								UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
							}
							common.PrintJsonPretty(*result)

						case autoAction.ActionType == AutoActionScaleIn:
							fmt.Println("[Action] "+ autoAction.ActionType)
						default:
					}

					mcisPolicyTmp.Status = AutoStatusReady
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Status + "],["+ v + "]")

				case mcisPolicyTmp.Status == AutoStatusOperating:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Status + "],["+ v + "]")
					//mcisPolicyTmp.Status = AutoStatusReady
					//UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)

				case mcisPolicyTmp.Status == AutoStatusTimeout:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Status + "],["+ v + "]")

				case mcisPolicyTmp.Status == AutoStatusError:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Status + "],["+ v + "]")
					mcisPolicyTmp.Status = AutoStatusReady
					UpdateMcisPolicyInfo(nsId, mcisPolicyTmp)

				case mcisPolicyTmp.Status == AutoStatusSuspended:
					fmt.Println("- PolicyStatus[" + mcisPolicyTmp.Status + "],["+ v + "]")

				default:
			}
	
		}

	}
	

}


func UpdateMcisPolicyInfo(nsId string, mcisPolicyInfoData McisPolicyInfo) {
	key := common.GenMcisPolicyKey(nsId, mcisPolicyInfoData.Id, "")
	val, _ := json.Marshal(mcisPolicyInfoData)
	err := common.CBStore.Put(string(key), string(val))
	if err != nil && !strings.Contains(err.Error(), common.CbStoreKeyNotFoundErrorString) {
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
	//fmt.Println("[Get MCIS Policy ID list]")
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
