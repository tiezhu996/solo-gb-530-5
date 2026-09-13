package dosebudget

import (
	"math"
	"testing"
	"time"
)

func TestMeasureActiveUsesHalfOpenWindow(t *testing.T) {
	from := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"before window", from.Add(-time.Second), false},
		{"at start inclusive", from, true},
		{"inside window", from.Add(24 * time.Hour), true},
		{"at end exclusive", to, false},
		{"after window", to.Add(time.Second), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := MeasureActive(from, to, test.at); got != test.want {
				t.Fatalf("MeasureActive(%v) = %v, want %v", test.at, got, test.want)
			}
		})
	}
	if MeasureActive(time.Time{}, to, from) {
		t.Fatal("zero effective_from unexpectedly active")
	}
	if MeasureActive(to, from, from) {
		t.Fatal("inverted window unexpectedly active")
	}
}

func TestCombineReductionFactorMultiplies(t *testing.T) {
	tests := []struct {
		name string
		pcts []float64
		want float64
	}{
		{"single measure", []float64{50}, 0.5},
		{"two measures compound", []float64{50, 50}, 0.25},
		{"three measures compound", []float64{20, 30, 50}, 0.28},
		{"small reduction", []float64{5}, 0.95},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CombineReductionFactor(test.pcts)
			if err != nil {
				t.Fatalf("CombineReductionFactor returned error: %v", err)
			}
			if math.Abs(got-test.want) > 1e-9 {
				t.Fatalf("factor = %v, want %v", got, test.want)
			}
		})
	}
	invalid := [][]float64{
		{},
		{0},
		{100},
		{-10},
		{math.NaN()},
		{math.Inf(1)},
	}
	for _, pcts := range invalid {
		if _, err := CombineReductionFactor(pcts); err == nil {
			t.Fatalf("pcts %v unexpectedly accepted", pcts)
		}
	}
}

func TestEvaluateScenarioSideBandsAndMargins(t *testing.T) {
	thresholds := Thresholds{AdministrativeLimitMSV: 12, LegalLimitMSV: 20, NearLegalRatio: 0.9, Version: "ALARA-2026.1"}
	tests := []struct {
		name          string
		periodDose    float64
		plannedDose   float64
		wantProjected float64
		wantBand      string
		wantAdminRem  float64
	}{
		{"within admin", 5, 2, 7, "within_admin", 5},
		{"above admin", 11, 2, 13, "above_admin", 0},
		{"near legal", 16, 2.5, 18.5, "near_legal", 0},
		{"above legal", 19, 2, 21, "above_legal", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := EvaluateScenarioSide(test.periodDose, test.plannedDose, thresholds)
			if err != nil {
				t.Fatalf("EvaluateScenarioSide returned error: %v", err)
			}
			if got.ProjectedDoseMSV != test.wantProjected || got.RiskBand != test.wantBand || got.RemainingAdminMSV != test.wantAdminRem {
				t.Fatalf("side = %+v, want projected=%v band=%v adminRem=%v", got, test.wantProjected, test.wantBand, test.wantAdminRem)
			}
		})
	}
	if _, err := EvaluateScenarioSide(-1, 0, thresholds); err == nil {
		t.Fatal("negative period dose unexpectedly accepted")
	}
	if _, err := EvaluateScenarioSide(0, math.NaN(), thresholds); err == nil {
		t.Fatal("NaN planned dose unexpectedly accepted")
	}
}
