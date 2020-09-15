package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNs(t *testing.T) {
	/*
		    expected := 1
		    actual := 0
			assert.Equal(t, expected, actual, "기대값과 결과값이 다릅니다.")
	*/

	nsName := "tb-unit-test"

	nsReq := NsReq{}
	nsReq.Name = nsName

	result, _ := CreateNs(&nsReq)
	assert.Equal(t, nsName, result.Name, "CreateNs 기대값과 결과값이 다릅니다.")

	result, _ = GetNs(nsName)
	assert.Equal(t, nsName, result.Name, "GetNs 기대값과 결과값이 다릅니다.")

	//result, _ := ListNs()

	//result, _ := ListNsId()

	resultBool, _ := CheckNs(nsName)
	assert.Equal(t, true, resultBool, "CheckNs 기대값과 결과값이 다릅니다.")

	resultErr := DelNs(nsName)
	assert.Nil(t, resultErr)
}
