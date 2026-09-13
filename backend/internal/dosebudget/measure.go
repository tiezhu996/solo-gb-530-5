package dosebudget

import (
	"errors"
	"fmt"
	"time"
)

var ErrInvalidMeasureInput = errors.New("invalid control measure input")

// MeasureSnapshot 是冻结进预算情景证据的单条控制措施快照。
type MeasureSnapshot struct {
	MeasureID            uint      `json:"measure_id"`
	MeasureCode          string    `json:"measure_code"`
	MeasureType          string    `json:"measure_type"`
	TaskCategory         string    `json:"task_category"`
	ExpectedReductionPct float64   `json:"expected_reduction_pct"`
	MeasureVersion       uint      `json:"measure_version"`
	EffectiveFrom        time.Time `json:"effective_from"`
	EffectiveTo          time.Time `json:"effective_to"`
	Basis                string    `json:"basis"`
}

// ScenarioSide 是采用前/后对照单侧的投影结果。
type ScenarioSide struct {
	PlannedDoseMSV    float64 `json:"planned_dose_msv"`
	ProjectedDoseMSV  float64 `json:"projected_dose_msv"`
	RemainingAdminMSV float64 `json:"remaining_admin_msv"`
	RemainingLegalMSV float64 `json:"remaining_legal_msv"`
	RiskBand          string  `json:"risk_band"`
}

// ScenarioEvidence 是随情景一起持久化的公式与版本证据。
type ScenarioEvidence struct {
	Formula           string  `json:"formula"`
	ReductionFormula  string  `json:"reduction_formula"`
	MeasureIDs        []uint  `json:"measure_ids"`
	ReductionFactor   float64 `json:"reduction_factor"`
	SavedDoseMSV      float64 `json:"saved_dose_msv"`
	FormulaVersion    string  `json:"formula_version"`
	ThresholdVersion  string  `json:"threshold_version"`
	BoundaryStatement string  `json:"boundary_statement"`
}

const (
	ScenarioFormula          = "baseline_planned_msv = estimated_rate_msvh * planned_minutes / 60; mitigated_planned_msv = baseline_planned_msv * reduction_factor; projected_dose_msv = period_dose_msv + planned_msv"
	ScenarioReductionFormula = "reduction_factor = product(1 - expected_reduction_pct / 100) over selected measures"
)

// MeasureActive 判断 at 是否落在半开生效窗口 [from, to) 内。
func MeasureActive(from, to, at time.Time) bool {
	from = from.UTC()
	to = to.UTC()
	at = at.UTC()
	return !from.IsZero() && to.After(from) && !at.Before(from) && at.Before(to)
}

// CombineReductionFactor 以连乘方式合成多条措施的折减系数，避免线性叠加超过 100%。
func CombineReductionFactor(pcts []float64) (float64, error) {
	if len(pcts) == 0 {
		return 0, fmt.Errorf("%w: at least one measure is required", ErrInvalidMeasureInput)
	}
	factor := 1.0
	for _, pct := range pcts {
		if !finite(pct) || pct <= 0 || pct >= 100 {
			return 0, fmt.Errorf("%w: expected reduction pct must be finite and in (0, 100)", ErrInvalidMeasureInput)
		}
		factor *= 1 - pct/100
	}
	if !finite(factor) || factor <= 0 {
		return 0, fmt.Errorf("%w: combined reduction factor overflow", ErrInvalidMeasureInput)
	}
	return roundDose(factor), nil
}

// RoundDose 对外暴露与投影一致的 1e-6 mSv 舍入，用于对照差值。
func RoundDose(value float64) float64 { return roundDose(value) }

// EvaluateScenarioSide 计算对照单侧的投影累计、余量与风险带。
func EvaluateScenarioSide(periodDose, plannedDose float64, thresholds Thresholds) (ScenarioSide, error) {
	if !finite(periodDose) || !finite(plannedDose) || periodDose < 0 || plannedDose < 0 {
		return ScenarioSide{}, fmt.Errorf("%w: period and planned dose must be finite and non-negative", ErrInvalidDoseInput)
	}
	projected := roundDose(periodDose + plannedDose)
	if !finite(projected) {
		return ScenarioSide{}, fmt.Errorf("%w: scenario projection overflow", ErrInvalidDoseInput)
	}
	decision, err := Evaluate(projected, thresholds)
	if err != nil {
		return ScenarioSide{}, err
	}
	return ScenarioSide{
		PlannedDoseMSV:    roundDose(plannedDose),
		ProjectedDoseMSV:  projected,
		RemainingAdminMSV: decision.RemainingAdminMSV,
		RemainingLegalMSV: decision.RemainingLegalMSV,
		RiskBand:          decision.RiskBand,
	}, nil
}
