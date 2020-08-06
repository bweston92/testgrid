/*
Copyright 2020 The TestGrid Authors.

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

package updater

import (
	"context"
	"reflect"
	"testing"
	"time"

	statepb "github.com/GoogleCloudPlatform/testgrid/pb/state"
)

func blank(n int) []string {
	var out []string
	for i := 0; i < n; i++ {
		out = append(out, "")
	}
	return out
}

func TestInflateGrid(t *testing.T) {
	var hours []time.Time
	when := time.Now().Round(time.Hour)
	for i := 0; i < 24; i++ {
		hours = append(hours, when)
		when = when.Add(time.Hour)
	}

	millis := func(t time.Time) float64 {
		return float64(t.Unix() * 1000)
	}

	cases := []struct {
		name     string
		grid     statepb.Grid
		earliest time.Time
		latest   time.Time
		expected []inflatedColumn
	}{
		{
			name: "basically works",
		},
		{
			name: "preserve column data",
			grid: statepb.Grid{
				Columns: []*statepb.Column{
					{
						Build:      "build",
						Name:       "name",
						Started:    5,
						Extra:      []string{"extra", "fun"},
						HotlistIds: "hot topic",
					},
					{
						Build:      "second build",
						Name:       "second name",
						Started:    10,
						Extra:      []string{"more", "gooder"},
						HotlistIds: "hot pocket",
					},
				},
			},
			latest: hours[23],
			expected: []inflatedColumn{
				{
					column: &statepb.Column{
						Build:      "build",
						Name:       "name",
						Started:    5,
						Extra:      []string{"extra", "fun"},
						HotlistIds: "hot topic",
					},
					cells: map[string]cell{},
				},
				{
					column: &statepb.Column{
						Build:      "second build",
						Name:       "second name",
						Started:    10,
						Extra:      []string{"more", "gooder"},
						HotlistIds: "hot pocket",
					},
					cells: map[string]cell{},
				},
			},
		},
		{
			name: "preserve row data",
			grid: statepb.Grid{
				Columns: []*statepb.Column{
					{
						Build:   "b1",
						Name:    "n1",
						Started: 1,
					},
					{
						Build:   "b2",
						Name:    "n2",
						Started: 2,
					},
				},
				Rows: []*statepb.Row{
					{
						Name: "name",
						Results: []int32{
							int32(statepb.Row_FAIL), 2,
						},
						CellIds:  []string{"this", "that"},
						Messages: []string{"important", "notice"},
						Icons:    []string{"I1", "I2"},
						Metric:   []string{"this", "that"},
						Metrics: []*statepb.Metric{
							{
								Indices: []int32{0, 2},
								Values:  []float64{0.1, 0.2},
							},
							{
								Name:    "override",
								Indices: []int32{1, 1},
								Values:  []float64{1.1},
							},
						},
					},
					{
						Name: "second",
						Results: []int32{
							int32(statepb.Row_PASS), 2,
						},
						CellIds:  blank(2),
						Messages: blank(2),
						Icons:    blank(2),
						Metric:   blank(2),
					},
				},
			},
			latest: hours[23],
			expected: []inflatedColumn{
				{
					column: &statepb.Column{
						Build:   "b1",
						Name:    "n1",
						Started: 1,
					},
					cells: map[string]cell{
						"name": {
							result:  statepb.Row_FAIL,
							cellID:  "this",
							message: "important",
							icon:    "I1",
							metrics: map[string]float64{
								"this": 0.1,
							},
						},
						"second": {
							result: statepb.Row_PASS,
						},
					},
				},
				{
					column: &statepb.Column{
						Build:   "b2",
						Name:    "n2",
						Started: 2,
					},
					cells: map[string]cell{
						"name": {
							result:  statepb.Row_FAIL,
							cellID:  "that",
							message: "notice",
							icon:    "I2",
							metrics: map[string]float64{
								"this":     0.2,
								"override": 1.1,
							},
						},
						"second": {
							result: statepb.Row_PASS,
						},
					},
				},
			},
		},
		{
			name: "drop latest columns",
			grid: statepb.Grid{
				Columns: []*statepb.Column{
					{
						Build:   "latest1",
						Started: millis(hours[23]),
					},
					{
						Build:   "latest2",
						Started: millis(hours[20]) + 1000,
					},
					{
						Build:   "keep1",
						Started: millis(hours[20]) + 999,
					},
					{
						Build:   "keep2",
						Started: millis(hours[10]),
					},
				},
				Rows: []*statepb.Row{
					{
						Name:     "hello",
						CellIds:  blank(4),
						Messages: blank(4),
						Icons:    blank(4),
						Results: []int32{
							int32(statepb.Row_RUNNING), 1,
							int32(statepb.Row_PASS), 1,
							int32(statepb.Row_FAIL), 1,
							int32(statepb.Row_FLAKY), 1,
						},
					},
					{
						Name:     "world",
						CellIds:  blank(4),
						Messages: blank(4),
						Icons:    blank(4),
						Results: []int32{
							int32(statepb.Row_PASS_WITH_SKIPS), 4,
						},
					},
				},
			},
			latest: hours[20],
			expected: []inflatedColumn{
				{
					column: &statepb.Column{
						Build:   "keep1",
						Started: millis(hours[20]) + 999,
					},
					cells: map[string]cell{
						"hello": {result: statepb.Row_FAIL},
						"world": {result: statepb.Row_PASS_WITH_SKIPS},
					},
				},
				{
					column: &statepb.Column{
						Build:   "keep2",
						Started: millis(hours[10]),
					},
					cells: map[string]cell{
						"hello": {result: statepb.Row_FLAKY},
						"world": {result: statepb.Row_PASS_WITH_SKIPS},
					},
				},
			},
		},
		{
			name: "drop old columns",
			grid: statepb.Grid{
				Columns: []*statepb.Column{
					{
						Build:   "current1",
						Started: millis(hours[20]),
					},
					{
						Build:   "current2",
						Started: millis(hours[10]),
					},
					{
						Build:   "old1",
						Started: millis(hours[10]) - 1,
					},
					{
						Build:   "old2",
						Started: millis(hours[0]),
					},
				},
				Rows: []*statepb.Row{
					{
						Name:     "hello",
						CellIds:  blank(4),
						Messages: blank(4),
						Icons:    blank(4),
						Results: []int32{
							int32(statepb.Row_RUNNING), 1,
							int32(statepb.Row_PASS), 1,
							int32(statepb.Row_FAIL), 1,
							int32(statepb.Row_FLAKY), 1,
						},
					},
					{
						Name:     "world",
						CellIds:  blank(4),
						Messages: blank(4),
						Icons:    blank(4),
						Results: []int32{
							int32(statepb.Row_PASS_WITH_SKIPS), 4,
						},
					},
				},
			},
			latest:   hours[23],
			earliest: hours[10],
			expected: []inflatedColumn{
				{
					column: &statepb.Column{
						Build:   "current1",
						Started: millis(hours[20]),
					},
					cells: map[string]cell{
						"hello": {result: statepb.Row_RUNNING},
						"world": {result: statepb.Row_PASS_WITH_SKIPS},
					},
				},
				{
					column: &statepb.Column{
						Build:   "current2",
						Started: millis(hours[10]),
					},
					cells: map[string]cell{
						"hello": {result: statepb.Row_PASS},
						"world": {result: statepb.Row_PASS_WITH_SKIPS},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := inflateGrid(&tc.grid, tc.earliest, tc.latest)
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("inflateGrid(%v) got %v, want %v", tc.grid, actual, tc.expected)
			}
		})

	}
}

func TestInflateRow(t *testing.T) {
	cases := []struct {
		name     string
		row      statepb.Row
		expected []cell
	}{
		{
			name: "basically works",
		},
		{
			name: "preserve cell ids",
			row: statepb.Row{
				CellIds:  []string{"cell-a", "cell-b"},
				Icons:    blank(2),
				Messages: blank(2),
				Results: []int32{
					int32(statepb.Row_PASS), 2,
				},
			},
			expected: []cell{
				{
					result: statepb.Row_PASS,
					cellID: "cell-a",
				},
				{
					result: statepb.Row_PASS,
					cellID: "cell-b",
				},
			},
		},
		{
			name: "only finished columns contain icons and messages",
			row: statepb.Row{
				CellIds: blank(8),
				Icons: []string{
					"F1", "~1", "~2",
				},
				Messages: []string{
					"fail", "flake-first", "flake-second",
				},
				Results: []int32{
					int32(statepb.Row_NO_RESULT), 2,
					int32(statepb.Row_FAIL), 1,
					int32(statepb.Row_NO_RESULT), 2,
					int32(statepb.Row_FLAKY), 2,
					int32(statepb.Row_NO_RESULT), 1,
				},
			},
			expected: []cell{
				{},
				{},
				{
					result:  statepb.Row_FAIL,
					icon:    "F1",
					message: "fail",
				},
				{},
				{},
				{
					result:  statepb.Row_FLAKY,
					icon:    "~1",
					message: "flake-first",
				},
				{
					result:  statepb.Row_FLAKY,
					icon:    "~2",
					message: "flake-second",
				},
				{},
			},
		},
		{
			name: "find metric name from row when missing",
			row: statepb.Row{
				CellIds:  blank(1),
				Icons:    blank(1),
				Messages: blank(1),
				Results: []int32{
					int32(statepb.Row_PASS), 1,
				},
				Metric: []string{"found-it"},
				Metrics: []*statepb.Metric{
					{
						Indices: []int32{0, 1},
						Values:  []float64{7},
					},
				},
			},
			expected: []cell{
				{
					result: statepb.Row_PASS,
					metrics: map[string]float64{
						"found-it": 7,
					},
				},
			},
		},
		{
			name: "prioritize local metric name",
			row: statepb.Row{
				CellIds:  blank(1),
				Icons:    blank(1),
				Messages: blank(1),
				Results: []int32{
					int32(statepb.Row_PASS), 1,
				},
				Metric: []string{"ignore-this"},
				Metrics: []*statepb.Metric{
					{
						Name:    "oh yeah",
						Indices: []int32{0, 1},
						Values:  []float64{7},
					},
				},
			},
			expected: []cell{
				{
					result: statepb.Row_PASS,
					metrics: map[string]float64{
						"oh yeah": 7,
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var actual []cell
			for r := range inflateRow(context.Background(), &tc.row) {
				actual = append(actual, r)
			}

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("inflateRow(%v) got %v, want %v", tc.row, actual, tc.expected)
			}
		})
	}
}

func TestInflateMetic(t *testing.T) {
	point := func(v float64) *float64 {
		return &v
	}
	cases := []struct {
		name     string
		indices  []int32
		values   []float64
		expected []*float64
	}{
		{
			name: "basically works",
		},
		{
			name:    "documented example with both values and holes works",
			indices: []int32{0, 2, 6, 4},
			values:  []float64{0.1, 0.2, 6.1, 6.2, 6.3, 6.4},
			expected: []*float64{
				point(0.1),
				point(0.2),
				nil,
				nil,
				nil,
				nil,
				point(6.1),
				point(6.2),
				point(6.3),
				point(6.4),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var actual []*float64
			metric := statepb.Metric{
				Name:    tc.name,
				Indices: tc.indices,
				Values:  tc.values,
			}
			for v := range inflateMetric(context.Background(), &metric) {
				actual = append(actual, v)
			}

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("inflateMetric(%v) got %v want %v", metric, actual, tc.expected)
			}
		})
	}
}

func TestInflateResults(t *testing.T) {
	cases := []struct {
		name     string
		results  []int32
		expected []statepb.Row_Result
	}{
		{
			name: "basically works",
		},
		{
			name: "first documented example with multiple values works",
			results: []int32{
				int32(statepb.Row_NO_RESULT), 3,
				int32(statepb.Row_PASS), 4,
			},
			expected: []statepb.Row_Result{
				statepb.Row_NO_RESULT,
				statepb.Row_NO_RESULT,
				statepb.Row_NO_RESULT,
				statepb.Row_PASS,
				statepb.Row_PASS,
				statepb.Row_PASS,
				statepb.Row_PASS,
			},
		},
		{
			name: "first item is the type",
			results: []int32{
				int32(statepb.Row_RUNNING), 1, // RUNNING == 4
			},
			expected: []statepb.Row_Result{
				statepb.Row_RUNNING,
			},
		},
		{
			name: "second item is the number of repetitions",
			results: []int32{
				int32(statepb.Row_PASS), 4, // Running == 1
			},
			expected: []statepb.Row_Result{
				statepb.Row_PASS,
				statepb.Row_PASS,
				statepb.Row_PASS,
				statepb.Row_PASS,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch := inflateResults(context.Background(), tc.results)
			var actual []statepb.Row_Result
			for r := range ch {
				actual = append(actual, r)
			}
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("inflateResults(%v) got %v, want %v", tc.results, actual, tc.expected)
			}
		})
	}
}