package util

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func MinInt32(x, y int32) int32 {
	if x < y {
		return x
	}
	return y
}

func MaxInt32(x, y int32) int32 {
	if x > y {
		return x
	}
	return y
}

func MinQuantity(x, y resource.Quantity) resource.Quantity {
	if x.Cmp(y) < 0 {
		return x
	}
	return y
}

func MaxQuantity(x, y resource.Quantity) resource.Quantity {
	if x.Cmp(y) > 0 {
		return x
	}
	return y
}

func ParseQuantity(q string) *resource.Quantity {
	res := resource.MustParse(q)
	return &res
}

func NewQuantity(q int64) *resource.Quantity {
	res := *resource.NewQuantity(q, resource.DecimalSI)
	return &res
}

func NewMilliQuantity(q int64) *resource.Quantity {
	res := *resource.NewMilliQuantity(q, resource.DecimalSI)
	return &res
}

func Int32ptr(v int32) *int32 {
	return &v
}

func DurationPtr(value int) *metav1.Duration {
	return &metav1.Duration{Duration: time.Duration(value)}

}

func NewChecksum(obj interface{}) (string, error) {
	marshalledObj, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	newChecksum := sha512.Sum512_224(marshalledObj)
	return hex.EncodeToString(newChecksum[:]), nil
}
