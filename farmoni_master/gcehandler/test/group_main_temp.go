package main

import (
	"context"
	"fmt"
	"log"
	_ "os"
	_ "strings"
	 "io/ioutil"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	_ "golang.org/x/oauth2/jwt"
	"google.golang.org/api/compute/v1"
	_ "google.golang.org/api/googleapi"
)


func main() {

	ctx := context.Background()

	// Use the downloaded JSON file in its entirety
	data, err := ioutil.ReadFile("/root/.gcp/credentials")
	if err != nil {
		log.Fatal(err)
	}

	conf, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/compute")
	if err != nil {
		log.Fatal(err)
	}

	/* Use the email & privateKey from the JSON file (good for ENV vars & CircleCI ;)
	email := os.Getenv("GCE_EMAIL")
	privateKey := os.Getenv("GCE_PRIVATE_KEY")
	privateKey = strings.Replace(privateKey, "\\n", "\n", -1)

	// this key will have a bunch of '\n's which must be removed and replaced with hard returns.
	// paste result into CircleCI env var

	conf := &jwt.Config{
		Email:      email,
		PrivateKey: []byte(privateKey),
		Scopes: []string{
			"https://www.googleapis.com/auth/compute",
		},
		TokenURL: google.JWTTokenURL,
	}*/

	client := conf.Client(oauth2.NoContext)
	computeService, err := compute.New(client)
	if err != nil {
		log.Fatal(err)
	}

        projectID := "ornate-course-236606"
        zone := "us-east1-b"

        rb := &compute.InstanceGroupManager{
                Name:               "group-1",
                Zone:               zone,
                InstanceTemplate:   "projects/ornate-course-236606/global/instanceTemplates/instance-template-1",
                TargetSize:         3,
        }

	resp, err := computeService.InstanceGroupManagers.Insert(projectID, zone, rb).Context(ctx).Do()
        if err != nil {
                log.Fatal(err)
        }

        // TODO: Change code below to process the `resp` object:
        fmt.Printf("%#v\n", resp)

}
