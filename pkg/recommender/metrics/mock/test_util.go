package mock

import (
	"github.com/pkg/errors"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"math"
	"time"
)

const (
	QueryWillReturnTrue  = "queryWillReturnTrue"
	QueryWillReturnFalse = "queryWillReturnFalse"
	IncorrectQueryExpr   = "incorrectQueryExpr"
)

func SimpleQueryFunction() func(string, time.Time) (model.Value, v1.Warnings, error) {
	return func(query string, _ time.Time) (model.Value, v1.Warnings, error) {

		var value int
		if query == IncorrectQueryExpr {
			return nil, v1.Warnings{}, errors.New("Incorrect query expression")
		} else if query == QueryWillReturnTrue {
			value = 1
		} else if query == QueryWillReturnFalse {
			value = 0
		} else {
			panic("Incorrect usage of SimpleQueryFunction in unit tests. Possible query expressions are: " +
				"\"queryWillReturnTrue\", \"queryWillReturnFalse\", \"incorrectQueryExpr\"")
		}

		res := model.Vector{
			{
				Metric: model.Metric{
					"label_name_1.1": "label_value_1.1",
					"label_name_1.2": "label_value_1.2",
				},
				Value:     model.SampleValue(value),
				Timestamp: model.Time(math.MinInt64),
			},
			{
				Metric: model.Metric{
					"label_name_2.1": "label_value_2.1",
					"label_name_2.2": "label_value_2.2",
				},
				Value:     model.SampleValue(value),
				Timestamp: model.Time(math.MaxInt64),
			},
		}
		return res, v1.Warnings{}, nil
	}
}

func SimpleRangedQueryFunction() func(string, v1.Range) (model.Value, v1.Warnings, error) {
	return func(query string, r v1.Range) (model.Value, v1.Warnings, error) {
		var value int
		if query == IncorrectQueryExpr {
			return nil, v1.Warnings{}, errors.New("Incorrect query expression")
		} else if query == QueryWillReturnTrue {
			value = 1
		} else if query == QueryWillReturnFalse {
			value = 0
		} else {
			panic("Incorrect usage of SimpleQueryFunction in tests. Possible query expressions are: " +
				"\"queryWillReturnTrue\", \"queryWillReturnFalse\", \"incorrectQueryExpr\"")
		}
		res := model.Matrix{
			{
				Metric: model.Metric{
					"label_name_1.1": "label_value_1.1",
					"label_name_1.2": "label_value_1.2",
				},
				Values: []model.SamplePair{
					{
						Timestamp: model.Time(math.MaxInt64 - 1),
						Value:     model.SampleValue(value),
					},
					{
						Timestamp: model.Time(math.MaxInt64),
						Value:     model.SampleValue(value),
					},
				},
			},
			{
				Metric: model.Metric{
					"label_name_2.1": "label_value_2.1",
					"label_name_2.2": "label_value_2.2",
				},
				Values: []model.SamplePair{
					{
						Timestamp: model.Time(math.MaxInt64 - 1),
						Value:     model.SampleValue(value),
					},
					{
						Timestamp: model.Time(math.MaxInt64),
						Value:     model.SampleValue(value),
					},
				},
			},
		}
		return res, v1.Warnings{}, nil
	}
}
