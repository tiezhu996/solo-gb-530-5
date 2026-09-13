import { Injectable, inject, signal } from '@angular/core';
import { tap } from 'rxjs';
import { MeasuresApi, ScenariosApi } from '../api/measures.api';
import { BudgetScenario, ControlMeasure, MeasureInput } from '../types/measure';

@Injectable({ providedIn: 'root' })
export class MeasuresStore {
  private readonly api = inject(MeasuresApi);
  private readonly scenariosApi = inject(ScenariosApi);
  readonly measures = signal<ControlMeasure[]>([]);
  readonly scenarios = signal<BudgetScenario[]>([]);
  readonly loading = signal(false);

  load(): void {
    this.loading.set(true);
    this.api.list().subscribe({
      next: response => { this.measures.set(response.data); this.loading.set(false); },
      error: () => this.loading.set(false),
    });
    this.scenariosApi.list().subscribe({
      next: response => this.scenarios.set(response.data),
      error: () => undefined,
    });
  }

  create(input: MeasureInput) {
    return this.api.create(input).pipe(tap(response => this.measures.update(items => [response.data, ...items])));
  }

  update(id: number, input: Omit<MeasureInput, 'measure_code' | 'enabled'> & { version: number }) {
    return this.api.update(id, input).pipe(tap(response => this.replace(response.data)));
  }

  setStatus(id: number, enabled: boolean, version: number) {
    return this.api.setStatus(id, enabled, version).pipe(tap(response => this.replace(response.data)));
  }

  createScenario(planId: number, measureIds: number[]) {
    return this.scenariosApi.create(planId, measureIds).pipe(
      tap(response => this.scenarios.update(items => [response.data, ...items])),
    );
  }

  private replace(measure: ControlMeasure): void {
    this.measures.update(items => items.map(item => item.id === measure.id ? measure : item));
  }
}
