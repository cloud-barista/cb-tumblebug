package mcis

import (
	//"encoding/json"
	//uuid "github.com/google/uuid"
	"fmt"
	"net/http"
	"os"

	//"fmt"
	//"net/http"
	//"io/ioutil"
	//"strconv"

	// CB-Store
	cbstore "github.com/cloud-barista/cb-store"
	"github.com/cloud-barista/cb-store/config"
	icbs "github.com/cloud-barista/cb-store/interfaces"
	"github.com/cloud-barista/cb-tumblebug/src/common"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	//"github.com/cloud-barista/cb-tumblebug/src/mcism"
	//"github.com/cloud-barista/cb-tumblebug/src/common"
)

// CB-Store
var cblog *logrus.Logger
var store icbs.Store
var SPIDER_URL string

func init() {
	cblog = config.Cblogger
	store = cbstore.GetStore()
	SPIDER_URL = os.Getenv("SPIDER_URL")
}

/*
func genUuid() string {
	return uuid.New().String()
}
*/

type mcirIds struct {
	CspImageId           string
	CspImageName         string
	CspSshKeyName        string
	Name                 string // Spec
	CspNetworkId         string
	CspNetworkName       string
	CspSecurityGroupId   string
	CspSecurityGroupName string
	CspPublicIpId        string
	CspPublicIpName      string
	CspVNicId            string
	CspVNicName          string

	ConnectionName string
}

func checkMcis(nsId string, mcisId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("checkMcis failed; nsId given is null.")
		return false, err
	} else if mcisId == "" {
		err := fmt.Errorf("checkMcis failed; mcisId given is null.")
		return false, err
	}

	fmt.Println("[Check mcis] " + mcisId)

	//key := "/ns/" + nsId + "/mcis/" + mcisId
	key := common.GenMcisKey(nsId, mcisId, "")
	//fmt.Println(key)

	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		return false, err
	}
	if keyValue != nil {
		return true, nil
	}
	return false, nil

}

func RestCheckMcis(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")

	exists, err := checkMcis(nsId, mcisId)

	type JsonTemplate struct {
		Exists bool `json:exists`
	}
	content := JsonTemplate{}
	content.Exists = exists

	if err != nil {
		cblog.Error(err)
		//mapA := map[string]string{"message": err.Error()}
		//return c.JSON(http.StatusFailedDependency, &mapA)
		return c.JSON(http.StatusNotFound, &content)
	}

	return c.JSON(http.StatusOK, &content)
}

func checkVm(nsId string, mcisId string, vmId string) (bool, error) {

	// Check parameters' emptiness
	if nsId == "" {
		err := fmt.Errorf("checkVm failed; nsId given is null.")
		return false, err
	} else if mcisId == "" {
		err := fmt.Errorf("checkVm failed; mcisId given is null.")
		return false, err
	} else if vmId == "" {
		err := fmt.Errorf("checkVm failed; vmId given is null.")
		return false, err
	}

	fmt.Println("[Check vm] " + mcisId + ", " + vmId)

	key := common.GenMcisKey(nsId, mcisId, vmId)
	//fmt.Println(key)

	keyValue, err := store.Get(key)
	if err != nil {
		cblog.Error(err)
		return false, err
	}
	if keyValue != nil {
		return true, nil
	}
	return false, nil

}

func RestCheckVm(c echo.Context) error {

	nsId := c.Param("nsId")
	mcisId := c.Param("mcisId")
	vmId := c.Param("vmId")

	exists, err := checkVm(nsId, mcisId, vmId)

	type JsonTemplate struct {
		Exists bool `json:exists`
	}
	content := JsonTemplate{}
	content.Exists = exists

	if err != nil {
		cblog.Error(err)
		//mapA := map[string]string{"message": err.Error()}
		//return c.JSON(http.StatusFailedDependency, &mapA)
		return c.JSON(http.StatusNotFound, &content)
	}

	return c.JSON(http.StatusOK, &content)
}
