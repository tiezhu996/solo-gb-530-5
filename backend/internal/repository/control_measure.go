package repository

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"radiation-dose-budget-control/backend/internal/model"
)

type ControlMeasureRepository struct{ db *gorm.DB }

func NewControlMeasureRepository(db *gorm.DB) *ControlMeasureRepository {
	return &ControlMeasureRepository{db: db}
}

func (repository *ControlMeasureRepository) WithDB(db *gorm.DB) *ControlMeasureRepository {
	return &ControlMeasureRepository{db: db}
}

func (repository *ControlMeasureRepository) Create(measure *model.ControlMeasure) error {
	if err := repository.db.Create(measure).Error; err != nil {
		return fmt.Errorf("create control measure: %w", err)
	}
	return nil
}

func (repository *ControlMeasureRepository) Find(id uint) (model.ControlMeasure, error) {
	var measure model.ControlMeasure
	if err := repository.db.First(&measure, id).Error; err != nil {
		return measure, fmt.Errorf("find control measure: %w", err)
	}
	return measure, nil
}

func (repository *ControlMeasureRepository) FindForUpdate(id uint) (model.ControlMeasure, error) {
	var measure model.ControlMeasure
	if err := repository.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&measure, id).Error; err != nil {
		return measure, fmt.Errorf("lock control measure: %w", err)
	}
	return measure, nil
}

func (repository *ControlMeasureRepository) FindByIDs(ids []uint) ([]model.ControlMeasure, error) {
	measures := make([]model.ControlMeasure, 0, len(ids))
	if err := repository.db.Where("id IN ?", ids).Find(&measures).Error; err != nil {
		return nil, fmt.Errorf("find control measures by ids: %w", err)
	}
	return measures, nil
}

func (repository *ControlMeasureRepository) List(page, pageSize int, taskCategory, measureType string, enabled *bool) ([]model.ControlMeasure, int64, error) {
	query := repository.db.Model(&model.ControlMeasure{})
	if taskCategory != "" {
		query = query.Where("task_category = ?", taskCategory)
	}
	if measureType != "" {
		query = query.Where("measure_type = ?", measureType)
	}
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count control measures: %w", err)
	}
	var measures []model.ControlMeasure
	if err := query.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&measures).Error; err != nil {
		return nil, 0, fmt.Errorf("list control measures: %w", err)
	}
	return measures, total, nil
}

func (repository *ControlMeasureRepository) Update(measure model.ControlMeasure, expectedVersion uint) error {
	result := repository.db.Model(&model.ControlMeasure{}).
		Where("id = ? AND version = ?", measure.ID, expectedVersion).
		Updates(map[string]any{
			"task_category": measure.TaskCategory, "measure_type": measure.MeasureType,
			"expected_reduction_pct": measure.ExpectedReductionPct,
			"effective_from":         measure.EffectiveFrom, "effective_to": measure.EffectiveTo,
			"basis": measure.Basis, "version": expectedVersion + 1,
		})
	if result.Error != nil {
		return fmt.Errorf("update control measure: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("update control measure: %w", ErrVersionConflict)
	}
	return nil
}

func (repository *ControlMeasureRepository) SetEnabled(id uint, expectedVersion uint, enabled bool) error {
	result := repository.db.Model(&model.ControlMeasure{}).
		Where("id = ? AND version = ?", id, expectedVersion).
		Updates(map[string]any{"enabled": enabled, "version": expectedVersion + 1})
	if result.Error != nil {
		return fmt.Errorf("toggle control measure: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("toggle control measure: %w", ErrVersionConflict)
	}
	return nil
}

type BudgetScenarioRepository struct{ db *gorm.DB }

func NewBudgetScenarioRepository(db *gorm.DB) *BudgetScenarioRepository {
	return &BudgetScenarioRepository{db: db}
}

func (repository *BudgetScenarioRepository) WithDB(db *gorm.DB) *BudgetScenarioRepository {
	return &BudgetScenarioRepository{db: db}
}

func (repository *BudgetScenarioRepository) Create(scenario *model.BudgetScenario) error {
	if err := repository.db.Create(scenario).Error; err != nil {
		return fmt.Errorf("create budget scenario: %w", err)
	}
	return nil
}

func (repository *BudgetScenarioRepository) Find(id uint) (model.BudgetScenario, error) {
	var scenario model.BudgetScenario
	if err := repository.db.First(&scenario, id).Error; err != nil {
		return scenario, fmt.Errorf("find budget scenario: %w", err)
	}
	return scenario, nil
}

func (repository *BudgetScenarioRepository) CountForPlan(planID uint) (int64, error) {
	var total int64
	if err := repository.db.Model(&model.BudgetScenario{}).Where("plan_id = ?", planID).Count(&total).Error; err != nil {
		return 0, fmt.Errorf("count plan budget scenarios: %w", err)
	}
	return total, nil
}

func (repository *BudgetScenarioRepository) List(page, pageSize int, planID, workerID uint) ([]model.BudgetScenario, int64, error) {
	query := repository.db.Model(&model.BudgetScenario{})
	if planID > 0 {
		query = query.Where("plan_id = ?", planID)
	}
	if workerID > 0 {
		query = query.Where("worker_id = ?", workerID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count budget scenarios: %w", err)
	}
	var scenarios []model.BudgetScenario
	if err := query.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&scenarios).Error; err != nil {
		return nil, 0, fmt.Errorf("list budget scenarios: %w", err)
	}
	return scenarios, total, nil
}
