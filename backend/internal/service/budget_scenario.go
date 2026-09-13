package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"radiation-dose-budget-control/backend/internal/constants"
	"radiation-dose-budget-control/backend/internal/dosebudget"
	"radiation-dose-budget-control/backend/internal/dto"
	"radiation-dose-budget-control/backend/internal/model"
	"radiation-dose-budget-control/backend/internal/repository"
)

type BudgetScenarioService struct {
	db         *gorm.DB
	scenarios  *repository.BudgetScenarioRepository
	measures   *repository.ControlMeasureRepository
	plans      *repository.WorkPermitPlanRepository
	workers    *repository.WorkerProfileRepository
	entries    *repository.ExposureEntryRepository
	audit      *AuditService
	nearRatio  float64
	thresholds string
}

func NewBudgetScenarioService(
	db *gorm.DB,
	scenarios *repository.BudgetScenarioRepository,
	measures *repository.ControlMeasureRepository,
	plans *repository.WorkPermitPlanRepository,
	workers *repository.WorkerProfileRepository,
	entries *repository.ExposureEntryRepository,
	audit *AuditService,
	nearRatio float64,
	thresholdVersion string,
) *BudgetScenarioService {
	return &BudgetScenarioService{
		db: db, scenarios: scenarios, measures: measures, plans: plans, workers: workers, entries: entries,
		audit: audit, nearRatio: nearRatio, thresholds: thresholdVersion,
	}
}

func (service *BudgetScenarioService) Create(request dto.CreateBudgetScenarioRequest, actor dto.Actor, requestID string) (dto.BudgetScenarioResponse, error) {
	measureIDs, err := uniqueMeasureIDs(request.MeasureIDs)
	if err != nil {
		return dto.BudgetScenarioResponse{}, err
	}
	var response dto.BudgetScenarioResponse
	err = service.db.Transaction(func(tx *gorm.DB) error {
		plans := service.plans.WithDB(tx)
		workers := service.workers.WithDB(tx)
		entries := service.entries.WithDB(tx)
		scenarios := service.scenarios.WithDB(tx)
		plan, err := plans.FindForUpdate(request.PlanID)
		if err != nil {
			return MapRepositoryError("work permit plan", err)
		}
		if plan.PermitStatus == constants.PermitStatusArchived {
			return Conflict("invalid_state", "archived plans cannot accept new budget scenarios", nil)
		}
		worker, err := workers.FindForUpdate(plan.WorkerID)
		if err != nil {
			return MapRepositoryError("plan worker", err)
		}
		measures, err := service.measures.WithDB(tx).FindByIDs(measureIDs)
		if err != nil {
			return Internal("could not load control measures", err)
		}
		if len(measures) != len(measureIDs) {
			return NotFound("control measure", fmt.Errorf("requested %d measures, found %d", len(measureIDs), len(measures)))
		}
		now := time.Now().UTC()
		if err := validateMeasureSelection(measures, plan, now); err != nil {
			return err
		}
		period, err := dosebudget.NewPeriod(worker.PeriodStart, now)
		if err != nil {
			return BadRequest("invalid_period", err.Error())
		}
		periodEntries, err := entries.PeriodEntries(worker.ID, period.Start, period.End)
		if err != nil {
			return Internal("could not load period exposure entries", err)
		}
		summary, err := dosebudget.SummarizeEntries(periodEntries)
		if err != nil {
			return BadRequest("invalid_exposure_chain", err.Error())
		}
		scenario, err := service.buildScenario(plan, worker, measures, summary.DoseMSV, scenarios, actor.ID)
		if err != nil {
			return err
		}
		if err := scenarios.Create(&scenario); err != nil {
			if repository.IsUniqueViolation(err) {
				return Conflict("duplicate_scenario_code", "scenario code already exists; retry the request", err)
			}
			return Internal("could not persist immutable budget scenario", err)
		}
		if err := service.audit.RecordTx(tx, actor, requestID, "scenario.created", "budget_scenario", auditID(scenario.ID),
			map[string]any{
				"plan_id": plan.ID, "measure_ids": measureIDs, "reduction_factor": scenario.ReductionFactor,
				"formula_version": scenario.FormulaVersion, "planning_only": true,
			}, nil, map[string]any{
				"scenario_code": scenario.ScenarioCode, "saved_dose_msv": scenario.SavedDoseMSV,
				"plan_version": scenario.PlanVersion, "worker_version": scenario.WorkerVersion,
			}); err != nil {
			return err
		}
		response = scenarioResponse(scenario, plan, worker)
		return nil
	})
	if err != nil {
		return dto.BudgetScenarioResponse{}, err
	}
	return response, nil
}

func (service *BudgetScenarioService) Get(id uint) (dto.BudgetScenarioResponse, error) {
	scenario, err := service.scenarios.Find(id)
	if err != nil {
		return dto.BudgetScenarioResponse{}, MapRepositoryError("budget scenario", err)
	}
	plan, err := service.plans.Find(scenario.PlanID)
	if err != nil {
		return dto.BudgetScenarioResponse{}, MapRepositoryError("scenario plan", err)
	}
	worker, err := service.workers.Find(scenario.WorkerID)
	if err != nil {
		return dto.BudgetScenarioResponse{}, MapRepositoryError("scenario worker", err)
	}
	return scenarioResponse(scenario, plan, worker), nil
}

func (service *BudgetScenarioService) List(page, pageSize int, planFilter, workerFilter string) ([]dto.BudgetScenarioResponse, dto.PageMeta, error) {
	planID, err := parseUintFilter(planFilter)
	if err != nil {
		return nil, dto.PageMeta{}, err
	}
	workerID, err := parseUintFilter(workerFilter)
	if err != nil {
		return nil, dto.PageMeta{}, err
	}
	scenarios, total, err := service.scenarios.List(page, pageSize, planID, workerID)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list budget scenarios", err)
	}
	responses := make([]dto.BudgetScenarioResponse, 0, len(scenarios))
	for _, scenario := range scenarios {
		plan, err := service.plans.Find(scenario.PlanID)
		if err != nil {
			return nil, dto.PageMeta{}, MapRepositoryError("scenario plan", err)
		}
		worker, err := service.workers.Find(scenario.WorkerID)
		if err != nil {
			return nil, dto.PageMeta{}, MapRepositoryError("scenario worker", err)
		}
		responses = append(responses, scenarioResponse(scenario, plan, worker))
	}
	return responses, pageMeta(page, pageSize, total), nil
}

func (service *BudgetScenarioService) buildScenario(
	plan model.WorkPermitPlan,
	worker model.WorkerProfile,
	measures []model.ControlMeasure,
	periodDose float64,
	scenarios *repository.BudgetScenarioRepository,
	createdBy uint,
) (model.BudgetScenario, error) {
	pcts := make([]float64, 0, len(measures))
	snapshots := make([]dosebudget.MeasureSnapshot, 0, len(measures))
	ids := make([]uint, 0, len(measures))
	for _, measure := range measures {
		pcts = append(pcts, measure.ExpectedReductionPct)
		ids = append(ids, measure.ID)
		snapshots = append(snapshots, dosebudget.MeasureSnapshot{
			MeasureID: measure.ID, MeasureCode: measure.MeasureCode, MeasureType: measure.MeasureType,
			TaskCategory: measure.TaskCategory, ExpectedReductionPct: measure.ExpectedReductionPct,
			MeasureVersion: measure.Version, EffectiveFrom: measure.EffectiveFrom, EffectiveTo: measure.EffectiveTo,
			Basis: measure.Basis,
		})
	}
	factor, err := dosebudget.CombineReductionFactor(pcts)
	if err != nil {
		return model.BudgetScenario{}, BadRequest("invalid_measure_reduction", err.Error())
	}
	baseline, err := dosebudget.CalculateProjection(periodDose, plan.EstimatedRateMSVH, plan.PlannedMinutes)
	if err != nil {
		return model.BudgetScenario{}, BadRequest("invalid_projection", err.Error())
	}
	thresholds := dosebudget.Thresholds{
		AdministrativeLimitMSV: worker.AdministrativeLimitMSV, LegalLimitMSV: worker.AnnualLimitMSV,
		NearLegalRatio: service.nearRatio, Version: service.thresholds,
	}
	baselineSide, err := dosebudget.EvaluateScenarioSide(periodDose, baseline.PlannedDoseMSV, thresholds)
	if err != nil {
		return model.BudgetScenario{}, BadRequest("invalid_thresholds", err.Error())
	}
	mitigatedSide, err := dosebudget.EvaluateScenarioSide(periodDose, baseline.PlannedDoseMSV*factor, thresholds)
	if err != nil {
		return model.BudgetScenario{}, BadRequest("invalid_thresholds", err.Error())
	}
	savedDose := dosebudget.RoundDose(baselineSide.PlannedDoseMSV - mitigatedSide.PlannedDoseMSV)
	evidence := dosebudget.ScenarioEvidence{
		Formula: dosebudget.ScenarioFormula, ReductionFormula: dosebudget.ScenarioReductionFormula,
		MeasureIDs: ids, ReductionFactor: factor, SavedDoseMSV: savedDose,
		FormulaVersion: constants.MeasureFormulaVersion, ThresholdVersion: service.thresholds,
		BoundaryStatement: dosebudget.BoundaryStatement,
	}
	measureSetJSON, err := json.Marshal(snapshots)
	if err != nil {
		return model.BudgetScenario{}, Internal("could not encode measure snapshots", err)
	}
	baselineJSON, err := json.Marshal(baselineSide)
	if err != nil {
		return model.BudgetScenario{}, Internal("could not encode baseline side", err)
	}
	mitigatedJSON, err := json.Marshal(mitigatedSide)
	if err != nil {
		return model.BudgetScenario{}, Internal("could not encode mitigated side", err)
	}
	comparisonJSON, err := json.Marshal(evidence)
	if err != nil {
		return model.BudgetScenario{}, Internal("could not encode scenario evidence", err)
	}
	count, err := scenarios.CountForPlan(plan.ID)
	if err != nil {
		return model.BudgetScenario{}, Internal("could not sequence plan scenarios", err)
	}
	return model.BudgetScenario{
		ScenarioCode: fmt.Sprintf("SCN-%s-%03d", plan.PlanCode, count+1),
		PlanID:       plan.ID, WorkerID: worker.ID,
		MeasureSetJSON: string(measureSetJSON), BaselineJSON: string(baselineJSON),
		MitigatedJSON: string(mitigatedJSON), ComparisonJSON: string(comparisonJSON),
		PeriodDoseMSV: periodDose, ReductionFactor: factor, SavedDoseMSV: savedDose,
		FormulaVersion: constants.MeasureFormulaVersion, ThresholdVersion: service.thresholds,
		PlanVersion: plan.Version, WorkerVersion: worker.Version, CreatedBy: createdBy,
	}, nil
}

func uniqueMeasureIDs(ids []uint) ([]uint, error) {
	seen := map[uint]bool{}
	unique := make([]uint, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			return nil, BadRequest("duplicate_measure", "measure_ids must be unique within one scenario")
		}
		seen[id] = true
		unique = append(unique, id)
	}
	return unique, nil
}

func validateMeasureSelection(measures []model.ControlMeasure, plan model.WorkPermitPlan, now time.Time) error {
	for _, measure := range measures {
		if !measure.Enabled {
			return Conflict("measure_not_enabled", fmt.Sprintf("measure %s is disabled and cannot be referenced", measure.MeasureCode), nil)
		}
		if !dosebudget.MeasureActive(measure.EffectiveFrom, measure.EffectiveTo, now) {
			return Conflict("measure_expired", fmt.Sprintf("measure %s is outside its effective window", measure.MeasureCode), nil)
		}
		if !strings.EqualFold(measure.TaskCategory, plan.TaskCategory) {
			return Conflict("measure_category_mismatch", fmt.Sprintf("measure %s targets task category %q, not %q", measure.MeasureCode, measure.TaskCategory, plan.TaskCategory), nil)
		}
	}
	return nil
}

func scenarioResponse(scenario model.BudgetScenario, plan model.WorkPermitPlan, worker model.WorkerProfile) dto.BudgetScenarioResponse {
	measures := []dosebudget.MeasureSnapshot{}
	_ = json.Unmarshal([]byte(scenario.MeasureSetJSON), &measures)
	baseline := dosebudget.ScenarioSide{}
	_ = json.Unmarshal([]byte(scenario.BaselineJSON), &baseline)
	mitigated := dosebudget.ScenarioSide{}
	_ = json.Unmarshal([]byte(scenario.MitigatedJSON), &mitigated)
	evidence := dosebudget.ScenarioEvidence{}
	_ = json.Unmarshal([]byte(scenario.ComparisonJSON), &evidence)
	measureViews := make([]dto.MeasureSnapshotView, 0, len(measures))
	for _, snapshot := range measures {
		measureViews = append(measureViews, dto.MeasureSnapshotView{
			MeasureID: snapshot.MeasureID, MeasureCode: snapshot.MeasureCode, MeasureType: snapshot.MeasureType,
			TaskCategory: snapshot.TaskCategory, ExpectedReductionPct: snapshot.ExpectedReductionPct,
			MeasureVersion: snapshot.MeasureVersion, EffectiveFrom: snapshot.EffectiveFrom,
			EffectiveTo: snapshot.EffectiveTo, Basis: snapshot.Basis,
		})
	}
	return dto.BudgetScenarioResponse{
		ID: scenario.ID, ScenarioCode: scenario.ScenarioCode,
		PlanID: plan.ID, PlanCode: plan.PlanCode, TaskCategory: plan.TaskCategory,
		WorkerID: worker.ID, WorkerCode: worker.WorkerCode, WorkerName: worker.DisplayName,
		PeriodDoseMSV: scenario.PeriodDoseMSV,
		Measures:      measureViews,
		Baseline: dto.ScenarioSideView{
			PlannedDoseMSV: baseline.PlannedDoseMSV, ProjectedDoseMSV: baseline.ProjectedDoseMSV,
			RemainingAdminMSV: baseline.RemainingAdminMSV, RemainingLegalMSV: baseline.RemainingLegalMSV,
			RiskBand: baseline.RiskBand,
		},
		Mitigated: dto.ScenarioSideView{
			PlannedDoseMSV: mitigated.PlannedDoseMSV, ProjectedDoseMSV: mitigated.ProjectedDoseMSV,
			RemainingAdminMSV: mitigated.RemainingAdminMSV, RemainingLegalMSV: mitigated.RemainingLegalMSV,
			RiskBand: mitigated.RiskBand,
		},
		ReductionFactor: scenario.ReductionFactor, SavedDoseMSV: scenario.SavedDoseMSV,
		Evidence: dto.ScenarioEvidenceView{
			Formula: evidence.Formula, ReductionFormula: evidence.ReductionFormula,
			MeasureIDs: evidence.MeasureIDs, ReductionFactor: evidence.ReductionFactor,
			SavedDoseMSV: evidence.SavedDoseMSV, FormulaVersion: evidence.FormulaVersion,
			ThresholdVersion: evidence.ThresholdVersion, BoundaryStatement: evidence.BoundaryStatement,
		},
		FormulaVersion: scenario.FormulaVersion, ThresholdVersion: scenario.ThresholdVersion,
		PlanVersion: scenario.PlanVersion, WorkerVersion: scenario.WorkerVersion, CreatedAt: scenario.CreatedAt,
	}
}
