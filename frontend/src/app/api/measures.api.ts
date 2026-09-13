import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { ApiEnvelope, PageEnvelope } from '../types/api';
import { BudgetScenario, ControlMeasure, MeasureInput } from '../types/measure';

@Injectable({ providedIn: 'root' })
export class MeasuresApi {
  private readonly http = inject(HttpClient);
  private readonly root = '/api/v1/measures';

  list() { return this.http.get<PageEnvelope<ControlMeasure>>(this.root, { params: { page_size: 100 } }); }
  get(id: number) { return this.http.get<ApiEnvelope<ControlMeasure>>(`${this.root}/${id}`); }
  create(input: MeasureInput) { return this.http.post<ApiEnvelope<ControlMeasure>>(this.root, input); }
  update(id: number, input: Omit<MeasureInput, 'measure_code' | 'enabled'> & { version: number }) {
    return this.http.put<ApiEnvelope<ControlMeasure>>(`${this.root}/${id}`, input);
  }
  setStatus(id: number, enabled: boolean, version: number) {
    return this.http.post<ApiEnvelope<ControlMeasure>>(`${this.root}/${id}/status`, { enabled, version });
  }
}

@Injectable({ providedIn: 'root' })
export class ScenariosApi {
  private readonly http = inject(HttpClient);
  private readonly root = '/api/v1/scenarios';

  list(planId?: number) {
    const params: Record<string, number> = { page_size: 50 };
    if (planId) params['plan_id'] = planId;
    return this.http.get<PageEnvelope<BudgetScenario>>(this.root, { params });
  }
  get(id: number) { return this.http.get<ApiEnvelope<BudgetScenario>>(`${this.root}/${id}`); }
  create(planId: number, measureIds: number[]) {
    return this.http.post<ApiEnvelope<BudgetScenario>>(this.root, { plan_id: planId, measure_ids: measureIds });
  }
}
