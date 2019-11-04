package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type cloudDriverRegisterRequestInfo struct {
	DriverName        string
	ProviderName      string
	DriverLibFileName string
}

type KeyValue struct {
	Key   string
	Value string
}

type cloudCredentialRegisterRequestInfo struct {
	CredentialName   string
	ProviderName     string
	KeyValueInfoList []KeyValue
}

type cloudRegionRegisterRequestInfo struct {
	RegionName       string
	ProviderName     string
	KeyValueInfoList []KeyValue
}

type cloudConnectionConfigCreateRequestInfo struct {
	ConfigName     string
	ProviderName   string
	DriverName     string
	CredentialName string
	RegionName     string
}

func registerCloudInfo(resource string, param interface{}) error {
	url := ""

	if resource == "driver" ||
		resource == "credential" ||
		resource == "region" ||
		resource == "connectionconfig" {
		url = SPIDER_URL + "/" + resource
	} else {
		err := fmt.Errorf("resource must be one of these: driver, credential, region, connectionconfig")
		return err
	}

	method := "POST"

	payload, _ := json.MarshalIndent(param, "", "  ")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, url, strings.NewReader(string(payload)))

	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		cblog.Error(err)
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	fmt.Println(string(body))
	if err != nil {
		cblog.Error(err)
		return err
	}

	fmt.Println("HTTP Status code " + strconv.Itoa(res.StatusCode))
	switch {
	case res.StatusCode >= 400 || res.StatusCode < 200:
		err := fmt.Errorf("HTTP Status code " + strconv.Itoa(res.StatusCode))
		fmt.Println("body: ", string(body))
		cblog.Error(err)
		return err
	}

	return nil
}
