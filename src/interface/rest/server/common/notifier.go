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

package common

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/core/notifier"
	"github.com/labstack/echo/v4"
)

// RestGetNotifier godoc
// @ID GetNotifier
// @Summary Get the registered notifier
// @Description Returns the notification channel (ntfy) CB-Tumblebug subscribes to. Tokens are masked unless showSecrets=true, which returns the full read/write tokens so others can subscribe (read token) or publish (write token).
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param showSecrets query boolean false "Return unmasked tokens" default(false)
// @Success 200 {object} model.NotifierInfo
// @Success 204 "no notifier is registered"
// @Router /notifier [get]
func RestGetNotifier(c echo.Context) error {
	showSecrets, _ := strconv.ParseBool(c.QueryParam("showSecrets"))
	info := notifier.Get()
	if showSecrets {
		info = notifier.Current()
	}
	if info == nil {
		return c.NoContent(http.StatusNoContent) // nothing registered; 204 keeps browser consoles quiet
	}
	return c.JSON(http.StatusOK, info)
}

// RestPutNotifier godoc
// @ID PutNotifier
// @Summary Register an existing ntfy server as the notifier
// @Description Registers an ntfy server and topic. CB-Tumblebug starts subscribing immediately; the write token is handed to VMs that report back.
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param notifierReq body model.NotifierRegisterReq true "ntfy server to register"
// @Success 200 {object} model.NotifierInfo
// @Failure 400 {object} model.SimpleMsg
// @Router /notifier [put]
func RestPutNotifier(c echo.Context) error {
	req := model.NotifierRegisterReq{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	if req.Url == "" || req.Topic == "" {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: "url and topic are required"})
	}
	info, err := notifier.Register(req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, info)
}

// RestDeleteNotifier godoc
// @ID DeleteNotifier
// @Summary Unregister the notifier
// @Description Stops subscribing and removes the registration. With deleteInfra=true, the Infra created by POST /notifier/ntfy is terminated and deleted as well.
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param deleteInfra query boolean false "Also delete the managed ntfy Infra" default(false)
// @Success 200 {object} model.SimpleMsg
// @Failure 404 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /notifier [delete]
func RestDeleteNotifier(c echo.Context) error {
	deleteInfra, _ := strconv.ParseBool(c.QueryParam("deleteInfra"))
	if err := notifier.DeleteNtfySet(deleteInfra); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "no notifier is registered" {
			status = http.StatusNotFound
		}
		return c.JSON(status, model.SimpleMsg{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, model.SimpleMsg{Message: "notifier unregistered"})
}

// RestPostNotifierNtfy godoc
// @ID PostNotifierNtfy
// @Summary Provision an ntfy server from a template and register it
// @Description Creates an Infra from the template (default infra-ntfy) in the given namespace (default: system), installs ntfy with TLS and token auth, registers it as the notifier, and runs a round-trip test. Takes a few minutes.
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param setReq body model.NotifierSetReq false "Provisioning options"
// @Success 200 {object} model.NotifierInfo
// @Failure 400 {object} model.SimpleMsg
// @Failure 500 {object} model.SimpleMsg
// @Router /notifier/ntfy [post]
func RestPostNotifierNtfy(c echo.Context) error {
	req := model.NotifierSetReq{}
	if c.Request().ContentLength != 0 {
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
		}
	}
	ctx := context.WithoutCancel(c.Request().Context())
	info, err := notifier.CreateNtfySet(ctx, req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.SimpleMsg{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, info)
}

// RestPostNotifierTest godoc
// @ID PostNotifierTest
// @Summary Run a publish/subscribe round-trip test on the notifier
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Success 200 {object} model.NotifierTestResult
// @Failure 404 {object} model.SimpleMsg
// @Router /notifier/test [post]
func RestPostNotifierTest(c echo.Context) error {
	if notifier.Get() == nil {
		return c.JSON(http.StatusNotFound, model.SimpleMsg{Message: "no notifier is registered"})
	}
	return c.JSON(http.StatusOK, notifier.Test(15*time.Second))
}

// RestPostNotifierPublish godoc
// @ID PostNotifierPublish
// @Summary Publish a message to the notifier topic
// @Description Publishes a JSON message with the given kind. data must be a JSON object; its fields are merged into the message.
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param publishReq body model.NotifierPublishReq true "Message to publish"
// @Success 200 {object} model.SimpleMsg
// @Failure 400 {object} model.SimpleMsg
// @Router /notifier/publish [post]
func RestPostNotifierPublish(c echo.Context) error {
	req := model.NotifierPublishReq{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	if req.Kind == "" {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: errors.New("kind is required").Error()})
	}
	id, err := notifier.Publish(req.Kind, req.Data)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, model.SimpleMsg{Message: "published: " + id})
}

// RestGetNotifierMessages godoc
// @ID GetNotifierMessages
// @Summary List recently received notifier messages
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param limit query int false "Maximum number of messages, newest first" default(50)
// @Success 200 {array} model.NotifierMessage
// @Router /notifier/messages [get]
func RestGetNotifierMessages(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200 // the notifier keeps at most 200 recent messages
	}
	return c.JSON(http.StatusOK, notifier.RecentMessages(limit))
}

// RestPostNotifierReader godoc
// @ID PostNotifierReader
// @Summary Create a read-only account on the managed ntfy server
// @Description Creates an ntfy user that can only read the notifier topic, for subscribing from the ntfy app, web UI or curl. The password is generated when omitted and returned only in this response.
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param readerReq body model.NotifierReaderReq true "Reader account"
// @Success 200 {object} model.NotifierReaderInfo
// @Failure 400 {object} model.SimpleMsg
// @Router /notifier/reader [post]
func RestPostNotifierReader(c echo.Context) error {
	req := model.NotifierReaderReq{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	info, err := notifier.AddReader(req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, info)
}

// RestGetNotifierReaders godoc
// @ID GetNotifierReaders
// @Summary List read-only accounts on the managed ntfy server
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Success 200 {array} string
// @Failure 400 {object} model.SimpleMsg
// @Router /notifier/reader [get]
func RestGetNotifierReaders(c echo.Context) error {
	readers, err := notifier.ListReaders()
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, readers)
}

// RestDeleteNotifierReader godoc
// @ID DeleteNotifierReader
// @Summary Delete a read-only account from the managed ntfy server
// @Tags [Admin] System Configuration
// @Accept  json
// @Produce  json
// @Param name path string true "Reader account name"
// @Success 200 {object} model.SimpleMsg
// @Failure 400 {object} model.SimpleMsg
// @Router /notifier/reader/{name} [delete]
func RestDeleteNotifierReader(c echo.Context) error {
	if err := notifier.RemoveReader(c.Param("name")); err != nil {
		return c.JSON(http.StatusBadRequest, model.SimpleMsg{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, model.SimpleMsg{Message: "reader removed"})
}
