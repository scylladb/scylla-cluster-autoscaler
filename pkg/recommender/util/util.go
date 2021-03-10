package util

import (
	"github.com/scylladb/scylla-operator-autoscaler/pkg/util"
	"k8s.io/apimachinery/pkg/api/resource"
	"math"
)

func CalculateMembers(current int32, min, max *int32, factor float64) int32 {
	val := int32(factor * float64(current))

	if max != nil {
		val = util.MinInt32(val, *max)
	}

	if min != nil {
		val = util.MaxInt32(val, *min)
	}

	return val
}

func CalculateCPU(current, min, max *resource.Quantity, factor float64) resource.Quantity {
	var val resource.Quantity

	// TODO keep original scale???
	// TODO check if this makes any sense
	// current * factor <= maxint64 -> current *= factor
	// current <= maxint64 / factor
	if current.Value() > (math.MaxInt64 / 1000) {
		if float64(current.Value()) <= math.MaxInt64/factor {
			val = *resource.NewQuantity(int64(factor*float64(current.Value())), current.Format)
		} else {
			val = *max
		}
	} else {
		if float64(current.MilliValue()) <= math.MaxInt64/factor {
			val = *resource.NewMilliQuantity(int64(factor*float64(current.MilliValue())), current.Format)
		} else {
			val = *max
		}
	}

	if max != nil {
		val = util.MinQuantity(val, *max)
	}

	if min != nil {
		val = util.MaxQuantity(val, *min)
	}

	return val
}

