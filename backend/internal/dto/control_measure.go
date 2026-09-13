package dto

import "time"

type CreateControlMeasureRequest struct {
	MeasureCode          string    `json:"measure_code" validate:"required,min=3,max=48"`
	TaskCategory         string    `json:"task_category" validate:"required,min=2,max=80"`
	MeasureType          string    `json:"measure_type" validate:"required,oneof=shielding distance rotation authorization"`
	ExpectedReductionPct float64   `json:"expected_reduction_pct" validate:"gt=0,lt=100"`
	EffectiveFrom        time.Time `json:"effective_from" validate:"required"`
	EffectiveTo          time.Time `json:"effective_to" validate:"required"`
	Basis                string    `json:"basis" validate:"required,min=3,max=500"`
	Enabled              bool      `json:"enabled"`
}

type UpdateControlMeasureRequest struct {
	TaskCategory         string    `json:"task_category" validate:"required,min=2,max=80"`
	MeasureType          string    `json:"measure_type" validate:"required,oneof=shielding distance rotation authorization"`
	ExpectedReductionPct float64   `json:"expected_reduction_pct" validate:"gt=0,lt=100"`
	EffectiveFrom        time.Time `json:"effective_from" validate:"required"`
	EffectiveTo          time.Time `json:"effective_to" validate:"required"`
	Basis                string    `json:"basis" validate:"required,min=3,max=500"`
	Version              uint      `json:"version" validate:"required,gt=0"`
}

type ControlMeasureStatusRequest struct {
	Enabled bool `json:"enabled"`
	Version uint `json:"version" validate:"required,gt=0"`
}

type ControlMeasureResponse struct {
	ID                   uint      `json:"id"`
	MeasureCode          string    `json:"measure_code"`
	TaskCategory         string    `json:"task_category"`
	MeasureType          string    `json:"measure_type"`
	ExpectedReductionPct float64   `json:"expected_reduction_pct"`
	EffectiveFrom        time.Time `json:"effective_from"`
	EffectiveTo          time.Time `json:"effective_to"`
	Basis                string    `json:"basis"`
	Enabled              bool      `json:"enabled"`
	Expired              bool      `json:"expired"`
	Version              uint      `json:"version"`
	CreatedBy            uint      `json:"created_by"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type CreateBudgetScenarioRequest struct {
	PlanID     uint   `json:"plan_id" validate:"required,gt=0"`
	MeasureIDs []uint `json:"measure_ids" validate:"required,min=1,max=12,dive,gt=0"`
}

type MeasureSnapshotView struct {
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

type ScenarioSideView struct {
	PlannedDoseMSV    float64 `json:"planned_dose_msv"`
	ProjectedDoseMSV  float64 `json:"projected_dose_msv"`
	RemainingAdminMSV float64 `json:"remaining_admin_msv"`
	RemainingLegalMSV float64 `json:"remaining_legal_msv"`
	RiskBand          string  `json:"risk_band"`
}

type ScenarioEvidenceView struct {
	Formula           string  `json:"formula"`
	ReductionFormula  string  `json:"reduction_formula"`
	MeasureIDs        []uint  `json:"measure_ids"`
	ReductionFactor   float64 `json:"reduction_factor"`
	SavedDoseMSV      float64 `json:"saved_dose_msv"`
	FormulaVersion    string  `json:"formula_version"`
	ThresholdVersion  string  `json:"threshold_version"`
	BoundaryStatement string  `json:"boundary_statement"`
}

type BudgetScenarioResponse struct {
	ID               uint                  `json:"id"`
	ScenarioCode     string                `json:"scenario_code"`
	PlanID           uint                  `json:"plan_id"`
	PlanCode         string                `json:"plan_code"`
	TaskCategory     string                `json:"task_category"`
	WorkerID         uint                  `json:"worker_id"`
	WorkerCode       string                `json:"worker_code"`
	WorkerName       string                `json:"worker_name"`
	PeriodDoseMSV    float64               `json:"period_dose_msv"`
	Measures         []MeasureSnapshotView `json:"measures"`
	Baseline         ScenarioSideView      `json:"baseline"`
	Mitigated        ScenarioSideView      `json:"mitigated"`
	ReductionFactor  float64               `json:"reduction_factor"`
	SavedDoseMSV     float64               `json:"saved_dose_msv"`
	Evidence         ScenarioEvidenceView  `json:"evidence"`
	FormulaVersion   string                `json:"formula_version"`
	ThresholdVersion string                `json:"threshold_version"`
	PlanVersion      uint                  `json:"plan_version"`
	WorkerVersion    uint                  `json:"worker_version"`
	CreatedAt        time.Time             `json:"created_at"`
}
