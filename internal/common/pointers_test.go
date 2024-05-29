package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToPtr(t *testing.T) {
	var valueInt int = 42
	gotInt := ToPtr(valueInt)
	assert.Equal(t, valueInt, *gotInt)

	var valueBool bool = true
	gotBool := ToPtr(valueBool)
	assert.Equal(t, valueBool, *gotBool)

	var valueUint64 uint64 = 1
	gotUint64 := ToPtr(valueUint64)
	assert.Equal(t, valueUint64, *gotUint64)

	var valueStr string = "the-greatest-test-value"
	gotStr := ToPtr(valueStr)
	assert.Equal(t, valueStr, *gotStr)

}

func TestPtrValueCopy(t *testing.T) {
	values := []interface{}{
		"string",
		10,
		42,
		true,
		false,
		uint16(16),
	}

	for idx := range values {
		t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
			ptr := &values[idx]
			ptrCopy := PtrValueCopy(ptr)

			assert.NotSame(t, ptr, ptrCopy)
			assert.Equal(t, *ptr, *ptrCopy)
		})
	}

	slc := []string{"1", "2", "3"}
	slcPtr := &slc
	slcPtrCopy := PtrValueCopy(slcPtr)
	assert.NotSame(t, slcPtr, slcPtrCopy)
	assert.Equal(t, *slcPtr, *slcPtrCopy)

	slc[2] = "4"
	assert.Equal(t, *slcPtr, slc)
	assert.Equal(t, *slcPtr, *slcPtrCopy)
}
