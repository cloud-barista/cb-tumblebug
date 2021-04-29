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
