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

// Package common is to include common methods for managing multi-cloud infra
package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNs(t *testing.T) {
	/*
		    expected := 1
		    actual := 0
			assert.Equal(t, expected, actual, "not approriate output.")
	*/

	nsName := "tb-unit-test"

	nsReq := NsReq{}
	nsReq.Name = nsName

	result, _ := CreateNs(&nsReq)
	assert.Equal(t, nsName, result.Name, "CreateNs: not approriate output.")

	result, _ = GetNs(nsName)
	assert.Equal(t, nsName, result.Name, "GetNs: not approriate output.")

	//result, _ := ListNs()

	//result, _ := ListNsId()

	resultBool, _ := CheckNs(nsName)
	assert.Equal(t, true, resultBool, "CheckNs: not approriate output.")

	resultErr := DelNs(nsName)
	assert.Nil(t, resultErr)
}
