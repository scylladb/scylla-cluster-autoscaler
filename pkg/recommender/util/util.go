package util

import (
	"github.com/scylladb/scylla-operator-autoscaler/pkg/util"
	"k8s.io/apimachinery/pkg/api/resource"
	"math"
)

func CalculateMembers(current int32, min, max *int32, factor float64) int32 {
	var val int32
	// if scaled current will overflow int32
	if float64(current) >= float64(math.MaxInt32)/factor {
		// then set max value
		val = math.MaxInt32
	} else {
		val = int32(factor * float64(current))
	}

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

	// if MilliValue won't overflow int64 and MilliValue*factor won't overflow int64
	if current.Value() <= (math.MaxInt64 / 1000) && float64(current.MilliValue()) <= math.MaxInt64/factor {
		// then MilliValue is scaled i.e. val = current.MilliValue() * factor
		val = *resource.NewMilliQuantity(int64(factor*float64(current.MilliValue())), current.Format)

	// else if MilliValue will overflow int64, but Value*factor won't overflow int64
	} else if float64(current.Value()) <= math.MaxInt64/factor {
		// then Value is scaled i.e. val = current.Value() * factor
		val = *resource.NewQuantity(int64(factor*float64(current.Value())), current.Format)

	// else if both will overflow int64
	} else {
		// then set max possible Quantity
		val = *resource.NewQuantity(math.MaxInt64, current.Format)
	}

	if max != nil {
		val = util.MinQuantity(val, *max)
	}

	if min != nil {
		val = util.MaxQuantity(val, *min)
	}

	return val
}

