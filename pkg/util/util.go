package util

import "k8s.io/apimachinery/pkg/api/resource"

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
