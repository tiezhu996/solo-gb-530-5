import { DoseBand } from './dose';

export type MeasureType = 'shielding' | 'distance' | 'rotation' | 'authorization';

export interface ControlMeasure {
  id: number;
  measure_code: string;
  task_category: string;
  measure_type: MeasureType;
  expected_reduction_pct: number;
  effective_from: string;
  effective_to: string;
  basis: string;
  enabled: boolean;
  expired: boolean;
  version: number;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface MeasureInput {
  measure_code: string;
  task_category: string;
  measure_type: MeasureType;
  expected_reduction_pct: number;
  effective_from: string;
  effective_to: string;
  basis: string;
  enabled: boolean;
}

export interface MeasureSnapshot {
  measure_id: number;
  measure_code: string;
  measure_type: MeasureType;
  task_category: string;
  expected_reduction_pct: number;
  measure_version: number;
  effective_from: string;
  effective_to: string;
  basis: string;
}

export interface ScenarioSide {
  planned_dose_msv: number;
  projected_dose_msv: number;
  remaining_admin_msv: number;
  remaining_legal_msv: number;
  risk_band: DoseBand;
}

export interface ScenarioEvidence {
  formula: string;
  reduction_formula: string;
  measure_ids: number[];
  reduction_factor: number;
  saved_dose_msv: number;
  formula_version: string;
  threshold_version: string;
  boundary_statement: string;
}

export interface BudgetScenario {
  id: number;
  scenario_code: string;
  plan_id: number;
  plan_code: string;
  task_category: string;
  worker_id: number;
  worker_code: string;
  worker_name: string;
  period_dose_msv: number;
  measures: MeasureSnapshot[];
  baseline: ScenarioSide;
  mitigated: ScenarioSide;
  reduction_factor: number;
  saved_dose_msv: number;
  evidence: ScenarioEvidence;
  formula_version: string;
  threshold_version: string;
  plan_version: number;
  worker_version: number;
  created_at: string;
}
