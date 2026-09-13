package service

import (
	"strconv"
	"strings"
	"time"

	"radiation-dose-budget-control/backend/internal/constants"
	"radiation-dose-budget-control/backend/internal/dosebudget"
	"radiation-dose-budget-control/backend/internal/dto"
	"radiation-dose-budget-control/backend/internal/model"
	"radiation-dose-budget-control/backend/internal/repository"
)

type ControlMeasureService struct {
	measures *repository.ControlMeasureRepository
	audit    *AuditService
}

func NewControlMeasureService(measures *repository.ControlMeasureRepository, audit *AuditService) *ControlMeasureService {
	return &ControlMeasureService{measures: measures, audit: audit}
}

func (service *ControlMeasureService) Create(request dto.CreateControlMeasureRequest, actor dto.Actor, requestID string) (dto.ControlMeasureResponse, error) {
	window, err := normalizeMeasureWindow(request.EffectiveFrom, request.EffectiveTo)
	if err != nil {
		return dto.ControlMeasureResponse{}, err
	}
	measure := model.ControlMeasure{
		MeasureCode:  strings.ToUpper(strings.TrimSpace(request.MeasureCode)),
		TaskCategory: strings.TrimSpace(request.TaskCategory), MeasureType: request.MeasureType,
		ExpectedReductionPct: request.ExpectedReductionPct,
		EffectiveFrom:        window[0], EffectiveTo: window[1],
		Basis: strings.TrimSpace(request.Basis), Enabled: request.Enabled,
		Version: 1, CreatedBy: actor.ID,
	}
	if err := service.measures.Create(&measure); err != nil {
		if repository.IsUniqueViolation(err) {
			return dto.ControlMeasureResponse{}, Conflict("duplicate_measure_code", "measure_code already exists", err)
		}
		return dto.ControlMeasureResponse{}, Internal("could not create control measure", err)
	}
	response := measureResponse(measure, time.Now().UTC())
	if err := service.audit.Record(actor, requestID, "measure.created", "control_measure", auditID(measure.ID),
		map[string]any{"measure_code": measure.MeasureCode, "measure_type": measure.MeasureType}, nil, measureAudit(measure)); err != nil {
		return dto.ControlMeasureResponse{}, err
	}
	return response, nil
}

func (service *ControlMeasureService) Update(id uint, request dto.UpdateControlMeasureRequest, actor dto.Actor, requestID string) (dto.ControlMeasureResponse, error) {
	before, err := service.measures.Find(id)
	if err != nil {
		return dto.ControlMeasureResponse{}, MapRepositoryError("control measure", err)
	}
	window, err := normalizeMeasureWindow(request.EffectiveFrom, request.EffectiveTo)
	if err != nil {
		return dto.ControlMeasureResponse{}, err
	}
	updated := before
	updated.TaskCategory = strings.TrimSpace(request.TaskCategory)
	updated.MeasureType = request.MeasureType
	updated.ExpectedReductionPct = request.ExpectedReductionPct
	updated.EffectiveFrom = window[0]
	updated.EffectiveTo = window[1]
	updated.Basis = strings.TrimSpace(request.Basis)
	if err := service.measures.Update(updated, request.Version); err != nil {
		if strings.Contains(err.Error(), repository.ErrVersionConflict.Error()) {
			return dto.ControlMeasureResponse{}, Conflict("version_conflict", "measure changed before update; refresh before retrying", err)
		}
		return dto.ControlMeasureResponse{}, Internal("could not update control measure", err)
	}
	updated.Version = request.Version + 1
	updated.UpdatedAt = time.Now().UTC()
	if err := service.audit.Record(actor, requestID, "measure.updated", "control_measure", auditID(id),
		map[string]any{"expected_version": request.Version}, measureAudit(before), measureAudit(updated)); err != nil {
		return dto.ControlMeasureResponse{}, err
	}
	return measureResponse(updated, time.Now().UTC()), nil
}

func (service *ControlMeasureService) SetEnabled(id uint, request dto.ControlMeasureStatusRequest, actor dto.Actor, requestID string) (dto.ControlMeasureResponse, error) {
	before, err := service.measures.Find(id)
	if err != nil {
		return dto.ControlMeasureResponse{}, MapRepositoryError("control measure", err)
	}
	now := time.Now().UTC()
	if request.Enabled && !dosebudget.MeasureActive(before.EffectiveFrom, before.EffectiveTo, now) {
		return dto.ControlMeasureResponse{}, Conflict("measure_expired", "an expired or not-yet-effective measure cannot be enabled", nil)
	}
	if before.Enabled == request.Enabled {
		return dto.ControlMeasureResponse{}, Conflict("invalid_state", "measure already has the requested enabled state", nil)
	}
	if err := service.measures.SetEnabled(id, request.Version, request.Enabled); err != nil {
		if strings.Contains(err.Error(), repository.ErrVersionConflict.Error()) {
			return dto.ControlMeasureResponse{}, Conflict("version_conflict", "measure changed before status update; refresh before retrying", err)
		}
		return dto.ControlMeasureResponse{}, Internal("could not change control measure status", err)
	}
	after := before
	after.Enabled = request.Enabled
	after.Version = request.Version + 1
	after.UpdatedAt = now
	action := "measure.disabled"
	if request.Enabled {
		action = "measure.enabled"
	}
	if err := service.audit.Record(actor, requestID, action, "control_measure", auditID(id),
		map[string]any{"expected_version": request.Version, "enabled": request.Enabled}, measureAudit(before), measureAudit(after)); err != nil {
		return dto.ControlMeasureResponse{}, err
	}
	return measureResponse(after, now), nil
}

func (service *ControlMeasureService) Get(id uint) (dto.ControlMeasureResponse, error) {
	measure, err := service.measures.Find(id)
	if err != nil {
		return dto.ControlMeasureResponse{}, MapRepositoryError("control measure", err)
	}
	return measureResponse(measure, time.Now().UTC()), nil
}

func (service *ControlMeasureService) List(page, pageSize int, taskCategory, measureType, enabledFilter string) ([]dto.ControlMeasureResponse, dto.PageMeta, error) {
	if measureType != "" && !constants.IsMeasureType(measureType) {
		return nil, dto.PageMeta{}, BadRequest("invalid_measure_type", "measure_type filter is not recognized")
	}
	var enabled *bool
	if strings.TrimSpace(enabledFilter) != "" {
		parsed, err := strconv.ParseBool(enabledFilter)
		if err != nil {
			return nil, dto.PageMeta{}, BadRequest("invalid_filter", "enabled filter must be true or false")
		}
		enabled = &parsed
	}
	measures, total, err := service.measures.List(page, pageSize, strings.TrimSpace(taskCategory), measureType, enabled)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list control measures", err)
	}
	now := time.Now().UTC()
	responses := make([]dto.ControlMeasureResponse, 0, len(measures))
	for _, measure := range measures {
		responses = append(responses, measureResponse(measure, now))
	}
	return responses, pageMeta(page, pageSize, total), nil
}

func normalizeMeasureWindow(from, to time.Time) ([2]time.Time, error) {
	from = from.UTC()
	to = to.UTC()
	if from.IsZero() || !to.After(from) {
		return [2]time.Time{}, BadRequest("invalid_window", "effective_to must be after effective_from")
	}
	return [2]time.Time{from, to}, nil
}

func measureResponse(measure model.ControlMeasure, now time.Time) dto.ControlMeasureResponse {
	return dto.ControlMeasureResponse{
		ID: measure.ID, MeasureCode: measure.MeasureCode, TaskCategory: measure.TaskCategory,
		MeasureType: measure.MeasureType, ExpectedReductionPct: measure.ExpectedReductionPct,
		EffectiveFrom: measure.EffectiveFrom, EffectiveTo: measure.EffectiveTo, Basis: measure.Basis,
		Enabled: measure.Enabled, Expired: !dosebudget.MeasureActive(measure.EffectiveFrom, measure.EffectiveTo, now),
		Version: measure.Version, CreatedBy: measure.CreatedBy, CreatedAt: measure.CreatedAt, UpdatedAt: measure.UpdatedAt,
	}
}

func measureAudit(measure model.ControlMeasure) map[string]any {
	return map[string]any{
		"id": measure.ID, "measure_code": measure.MeasureCode, "task_category": measure.TaskCategory,
		"measure_type": measure.MeasureType, "expected_reduction_pct": measure.ExpectedReductionPct,
		"effective_from": measure.EffectiveFrom, "effective_to": measure.EffectiveTo,
		"enabled": measure.Enabled, "version": measure.Version,
	}
}
