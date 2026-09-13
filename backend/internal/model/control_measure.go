package model

import "time"

type ControlMeasure struct {
	ID                   uint      `gorm:"primaryKey"`
	MeasureCode          string    `gorm:"size:48;uniqueIndex;not null"`
	TaskCategory         string    `gorm:"size:80;index;not null"`
	MeasureType          string    `gorm:"size:24;index;not null"`
	ExpectedReductionPct float64   `gorm:"not null"`
	EffectiveFrom        time.Time `gorm:"not null"`
	EffectiveTo          time.Time `gorm:"not null"`
	Basis                string    `gorm:"size:500;not null"`
	Enabled              bool      `gorm:"not null"`
	Version              uint      `gorm:"not null;default:1"`
	CreatedBy            uint      `gorm:"index;not null"`
	CreatedAt            time.Time `gorm:"not null"`
	UpdatedAt            time.Time `gorm:"not null"`
}

type BudgetScenario struct {
	ID               uint      `gorm:"primaryKey"`
	ScenarioCode     string    `gorm:"size:64;uniqueIndex;not null"`
	PlanID           uint      `gorm:"index;not null"`
	WorkerID         uint      `gorm:"index;not null"`
	MeasureSetJSON   string    `gorm:"type:text;not null"`
	BaselineJSON     string    `gorm:"type:text;not null"`
	MitigatedJSON    string    `gorm:"type:text;not null"`
	ComparisonJSON   string    `gorm:"type:text;not null"`
	PeriodDoseMSV    float64   `gorm:"not null"`
	ReductionFactor  float64   `gorm:"not null"`
	SavedDoseMSV     float64   `gorm:"not null"`
	FormulaVersion   string    `gorm:"size:40;not null"`
	ThresholdVersion string    `gorm:"size:40;not null"`
	PlanVersion      uint      `gorm:"not null"`
	WorkerVersion    uint      `gorm:"not null"`
	CreatedBy        uint      `gorm:"index;not null"`
	CreatedAt        time.Time `gorm:"index;not null"`
}
