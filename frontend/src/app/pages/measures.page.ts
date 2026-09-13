import { ChangeDetectionStrategy, Component, OnInit, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { finalize } from 'rxjs';
import { MeasuresStore } from '../stores/measures.store';
import { PlansStore } from '../stores/plans.store';
import { useAuth } from '../hooks/use-auth';
import { DoseBandBadgeComponent } from '../components/common/dose-band-badge.component';
import { SafetyBoundaryBannerComponent } from '../components/common/safety-boundary-banner.component';
import { apiErrorMessage, inputToUTC, utcNowInput } from '../utils/api-error';
import { BudgetScenario, ControlMeasure, MeasureType } from '../types/measure';
import { WorkPermitPlan } from '../types/permit';

const MEASURE_TYPES: { value: MeasureType; label: string }[] = [
  { value: 'shielding', label: 'Shielding · 屏蔽' },
  { value: 'distance', label: 'Distance · 距离' },
  { value: 'rotation', label: 'Rotation · 轮换' },
  { value: 'authorization', label: 'Authorization · 授权' },
];

@Component({
  standalone: true,
  imports: [
    CommonModule, ReactiveFormsModule, MatButtonModule, MatFormFieldModule, MatInputModule,
    MatSelectModule, DoseBandBadgeComponent, SafetyBoundaryBannerComponent,
  ],
  template: `
    <div class="page">
      <header class="page-head">
        <div><span class="eyebrow">ALARA control measures</span><h1>Budget control measures</h1><p>Register shielding, distance, rotation and authorization controls per task category, then compare a plan before and after adoption.</p></div>
        <button *ngIf="auth.canPlan()" mat-flat-button color="primary" (click)="newMeasure()">{{ formOpen() ? 'Reset form' : 'New measure' }}</button>
      </header>
      <app-safety-boundary-banner title="Planning estimate, not a field measurement" detail="Expected reductions are planning assumptions frozen into scenario evidence. They never replace survey data, RPO judgment or the site work authorization process." />
      <p class="error-banner" *ngIf="error()">{{ error() }}</p>

      <form *ngIf="formOpen()" class="inline-form" [formGroup]="form" (ngSubmit)="save()">
        <mat-form-field class="span-3" appearance="outline"><mat-label>Measure code</mat-label><input matInput formControlName="measure_code" [readonly]="!!editing()"></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Task category</mat-label><input matInput formControlName="task_category"></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Type</mat-label><mat-select formControlName="measure_type"><mat-option *ngFor="let type of measureTypes" [value]="type.value">{{ type.label }}</mat-option></mat-select></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Expected reduction %</mat-label><input matInput type="number" min="1" max="99" step="0.5" formControlName="expected_reduction_pct"></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Effective from</mat-label><input matInput type="datetime-local" formControlName="effective_from"></mat-form-field>
        <mat-form-field class="span-3" appearance="outline"><mat-label>Effective to</mat-label><input matInput type="datetime-local" formControlName="effective_to"></mat-form-field>
        <mat-form-field class="span-4" appearance="outline"><mat-label>Basis / reference</mat-label><textarea matInput rows="2" formControlName="basis"></textarea></mat-form-field>
        <mat-form-field class="span-2" appearance="outline"><mat-label>Status</mat-label><mat-select formControlName="enabled" [disabled]="!!editing()"><mat-option [value]="true">Enabled</mat-option><mat-option [value]="false">Disabled</mat-option></mat-select></mat-form-field>
        <div class="span-12 form-actions"><button mat-button type="button" (click)="closeForm()">Cancel</button><button mat-flat-button color="primary" type="submit" [disabled]="form.invalid || saving()">{{ saving() ? 'Saving' : editing() ? 'Update measure' : 'Register measure' }}</button></div>
      </form>

      <div class="section-title"><h2>Measure register</h2><span>{{ store.measures().length }} measures · expired and disabled rows cannot be referenced</span></div>
      <div class="surface">
        <table class="data-table">
          <thead><tr><th>Measure</th><th>Category</th><th>Reduction</th><th>Effective window</th><th>Basis</th><th>Status</th><th></th></tr></thead>
          <tbody>
            <tr *ngFor="let measure of store.measures()" [class.selected]="editing()?.id === measure.id">
              <td><strong>{{ measure.measure_code }}</strong><br><span class="measure-type" [attr.data-type]="measure.measure_type">{{ measure.measure_type }}</span></td>
              <td>{{ measure.task_category }}</td>
              <td class="number">−{{ measure.expected_reduction_pct | number:'1.0-1' }}%</td>
              <td class="window">{{ measure.effective_from | date:'yyyy-MM-dd' }} → {{ measure.effective_to | date:'yyyy-MM-dd' }}<br><span *ngIf="measure.expired" class="expired-chip">expired</span></td>
              <td class="basis">{{ measure.basis }}</td>
              <td><span class="plan-status" [attr.data-status]="measure.enabled ? 'enabled' : 'disabled'">{{ measure.enabled ? 'enabled' : 'disabled' }}</span><br><span class="code muted">v{{ measure.version }}</span></td>
              <td class="actions" *ngIf="auth.canPlan()">
                <button mat-button (click)="edit(measure)">Edit</button>
                <button mat-button (click)="toggle(measure)">{{ measure.enabled ? 'Disable' : 'Enable' }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="section-title"><h2>Scenario comparison</h2><span>select a plan, adopt measures, review before / after</span></div>
      <div class="surface scenario-panel">
        <div class="scenario-controls">
          <mat-form-field appearance="outline" class="plan-select"><mat-label>Plan scenario</mat-label>
            <mat-select [value]="scenarioPlanId()" (selectionChange)="pickPlan($event.value)">
              <mat-option *ngFor="let plan of scenarioPlans()" [value]="plan.id">{{ plan.plan_code }} · {{ plan.task_category }} · {{ plan.worker_name }}</mat-option>
            </mat-select>
          </mat-form-field>
          <div class="factor-preview" *ngIf="selectedIds().size > 0">
            <strong>{{ combinedFactor() | number:'1.2-2' }}</strong><span>combined factor</span>
            <strong>{{ (1 - combinedFactor()) * 100 | number:'1.1-1' }}%</strong><span>expected reduction</span>
          </div>
        </div>
        <div class="measure-picker" *ngIf="scenarioPlanId()">
          <p class="muted" *ngIf="categoryMeasures().length === 0">No registered measures target task category “{{ selectedPlan()?.task_category }}”.</p>
          <label class="measure-option" *ngFor="let measure of categoryMeasures()" [class.blocked]="!referencable(measure)">
            <input type="checkbox" [checked]="selectedIds().has(measure.id)" [disabled]="!referencable(measure)" (change)="toggleSelection(measure.id)">
            <span class="option-code">{{ measure.measure_code }}</span>
            <span class="measure-type" [attr.data-type]="measure.measure_type">{{ measure.measure_type }}</span>
            <span class="number">−{{ measure.expected_reduction_pct | number:'1.0-1' }}%</span>
            <span class="muted reason" *ngIf="!measure.enabled">disabled</span>
            <span class="muted reason" *ngIf="measure.enabled && measure.expired">expired</span>
          </label>
        </div>
        <div class="form-actions" *ngIf="scenarioPlanId()">
          <button mat-flat-button color="primary" [disabled]="selectedIds().size === 0 || scenarioSaving() || !auth.canPlan()" (click)="runScenario()">{{ scenarioSaving() ? 'Comparing' : 'Generate before / after comparison' }}</button>
        </div>
        <div class="comparison" *ngIf="comparison() as result">
          <div class="side">
            <h3>Before adoption</h3>
            <dl><dt>Planned dose</dt><dd>{{ result.baseline.planned_dose_msv | number:'1.3-3' }} mSv</dd>
            <dt>Projected total</dt><dd>{{ result.baseline.projected_dose_msv | number:'1.3-3' }} mSv</dd>
            <dt>Admin margin</dt><dd>{{ result.baseline.remaining_admin_msv | number:'1.3-3' }} mSv</dd>
            <dt>Legal margin</dt><dd>{{ result.baseline.remaining_legal_msv | number:'1.3-3' }} mSv</dd>
            <dt>Risk band</dt><dd><app-dose-band-badge [band]="result.baseline.risk_band" /></dd></dl>
          </div>
          <div class="delta">
            <strong>−{{ result.saved_dose_msv | number:'1.3-3' }} mSv</strong>
            <span>factor {{ result.reduction_factor | number:'1.2-2' }}</span>
          </div>
          <div class="side">
            <h3>After adoption</h3>
            <dl><dt>Planned dose</dt><dd>{{ result.mitigated.planned_dose_msv | number:'1.3-3' }} mSv</dd>
            <dt>Projected total</dt><dd>{{ result.mitigated.projected_dose_msv | number:'1.3-3' }} mSv</dd>
            <dt>Admin margin</dt><dd>{{ result.mitigated.remaining_admin_msv | number:'1.3-3' }} mSv</dd>
            <dt>Legal margin</dt><dd>{{ result.mitigated.remaining_legal_msv | number:'1.3-3' }} mSv</dd>
            <dt>Risk band</dt><dd><app-dose-band-badge [band]="result.mitigated.risk_band" /></dd></dl>
          </div>
          <footer class="evidence">
            <span>{{ result.scenario_code }} · formula {{ result.formula_version }} · thresholds {{ result.threshold_version }} · plan v{{ result.plan_version }} · worker v{{ result.worker_version }}</span>
            <span>{{ result.evidence.reduction_formula }}</span>
            <span>{{ result.evidence.boundary_statement }}</span>
          </footer>
        </div>
      </div>

      <div class="section-title"><h2>Scenario register</h2><span>{{ store.scenarios().length }} immutable comparisons</span></div>
      <div class="surface">
        <table class="data-table">
          <thead><tr><th>Scenario</th><th>Plan</th><th>Measures</th><th>Before → after</th><th>Saved</th><th>Created</th></tr></thead>
          <tbody>
            <tr *ngFor="let scenario of store.scenarios()">
              <td><strong>{{ scenario.scenario_code }}</strong><br><span class="code muted">{{ scenario.formula_version }}</span></td>
              <td>{{ scenario.plan_code }}<br><span class="muted">{{ scenario.worker_name }}</span></td>
              <td><span class="control" *ngFor="let measure of scenario.measures">{{ measure.measure_code }} v{{ measure.measure_version }}</span></td>
              <td class="bands"><app-dose-band-badge [band]="scenario.baseline.risk_band" /> → <app-dose-band-badge [band]="scenario.mitigated.risk_band" /></td>
              <td class="number">−{{ scenario.saved_dose_msv | number:'1.3-3' }} mSv</td>
              <td class="muted">{{ scenario.created_at | date:'yyyy-MM-dd HH:mm' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  `,
  styles: [`
    .number { font-variant-numeric: tabular-nums; white-space: nowrap; }
    .window { white-space: nowrap; font-variant-numeric: tabular-nums; }
    .basis { max-width: 260px; font-size: 11px; color: var(--muted); }
    .actions { white-space: nowrap; }
    .measure-type { display: inline-block; margin-top: 2px; padding: 2px 6px; border-radius: 2px; font-size: 10px; font-weight: 700; background: #e7ece7; }
    .measure-type[data-type="shielding"] { background: #dfe9f4; color: #1d4e7e; }
    .measure-type[data-type="distance"] { background: #e5f1eb; color: #185847; }
    .measure-type[data-type="rotation"] { background: #f3e8d8; color: #76510b; }
    .measure-type[data-type="authorization"] { background: #ece4f2; color: #5b3a86; }
    .expired-chip { display: inline-block; margin-top: 2px; padding: 1px 6px; border-radius: 2px; font-size: 10px; font-weight: 800; color: #8c2929; background: #f8dfde; }
    .plan-status { display: inline-flex; padding: 3px 7px; border: 1px solid #b6c2ba; border-radius: 3px; font-size: 10px; font-weight: 800; }
    .plan-status[data-status="enabled"] { color: #185847; border-color: #8eb8a8; background: #e5f1eb; }
    .plan-status[data-status="disabled"] { color: #8c2929; border-color: #d89591; background: #f8dfde; }
    .control { display: inline-block; margin: 2px 4px 2px 0; padding: 2px 6px; background: #e7ece7; border-radius: 2px; font-size: 10px; }
    .scenario-panel { padding: 16px; margin-bottom: 28px; }
    .scenario-controls { display: flex; flex-wrap: wrap; gap: 18px; align-items: center; }
    .plan-select { min-width: 340px; }
    .factor-preview { display: flex; gap: 14px; align-items: baseline; padding: 8px 14px; border-left: 3px solid #c47d10; }
    .factor-preview strong { font-size: 20px; font-variant-numeric: tabular-nums; }
    .factor-preview span { color: var(--muted); font-size: 10px; text-transform: uppercase; }
    .measure-picker { display: grid; gap: 6px; margin: 12px 0; }
    .measure-option { display: flex; gap: 10px; align-items: center; padding: 7px 10px; border: 1px solid #d7ded7; border-radius: 3px; font-size: 12px; }
    .measure-option.blocked { opacity: 0.55; }
    .option-code { font-weight: 700; }
    .reason { font-size: 10px; text-transform: uppercase; }
    .comparison { display: grid; grid-template-columns: 1fr auto 1fr; gap: 18px; margin-top: 16px; padding-top: 16px; border-top: 1px solid #d7ded7; }
    .side h3 { margin: 0 0 8px; font-size: 12px; text-transform: uppercase; letter-spacing: 0.04em; color: var(--muted); }
    .side dl { display: grid; grid-template-columns: auto 1fr; gap: 4px 14px; margin: 0; font-size: 12px; }
    .side dt { color: var(--muted); } .side dd { margin: 0; font-variant-numeric: tabular-nums; text-align: right; }
    .delta { align-self: center; text-align: center; }
    .delta strong { display: block; font-size: 22px; color: #185847; font-variant-numeric: tabular-nums; }
    .delta span { font-size: 10px; color: var(--muted); text-transform: uppercase; }
    .evidence { grid-column: 1 / -1; display: grid; gap: 3px; padding-top: 10px; border-top: 1px dashed #d7ded7; font-size: 10px; color: var(--muted); }
    .bands { white-space: nowrap; }
    @media (max-width: 900px) { .comparison { grid-template-columns: 1fr; } }
  `],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MeasuresPage implements OnInit {
  readonly store = inject(MeasuresStore);
  readonly plans = inject(PlansStore);
  readonly auth = useAuth();
  private readonly builder = new FormBuilder().nonNullable;
  readonly measureTypes = MEASURE_TYPES;
  readonly editing = signal<ControlMeasure | null>(null);
  readonly formOpen = signal(false);
  readonly saving = signal(false);
  readonly error = signal('');
  readonly scenarioPlanId = signal(0);
  readonly selectedIds = signal<Set<number>>(new Set());
  readonly scenarioSaving = signal(false);
  readonly comparison = signal<BudgetScenario | null>(null);
  readonly scenarioPlans = computed(() => this.plans.plans().filter(plan => plan.permit_status !== 'archived'));
  readonly selectedPlan = computed(() => this.scenarioPlans().find(plan => plan.id === this.scenarioPlanId()) ?? null);
  readonly categoryMeasures = computed(() => {
    const category = this.selectedPlan()?.task_category.toLowerCase();
    if (!category) return [];
    return this.store.measures().filter(measure => measure.task_category.toLowerCase() === category);
  });
  readonly combinedFactor = computed(() => {
    let factor = 1;
    for (const measure of this.categoryMeasures()) {
      if (this.selectedIds().has(measure.id)) factor *= 1 - measure.expected_reduction_pct / 100;
    }
    return factor;
  });
  readonly form = this.builder.group({
    measure_code: ['ALARA-', [Validators.required, Validators.minLength(3)]],
    task_category: ['', [Validators.required, Validators.minLength(2)]],
    measure_type: ['shielding' as MeasureType, Validators.required],
    expected_reduction_pct: [25, [Validators.required, Validators.min(0.5), Validators.max(99)]],
    effective_from: [utcNowInput(), Validators.required],
    effective_to: ['', Validators.required],
    basis: ['', [Validators.required, Validators.minLength(3)]],
    enabled: [true, Validators.required],
  });

  ngOnInit(): void { this.store.load(); this.plans.load(); }

  newMeasure(): void {
    this.editing.set(null); this.formOpen.set(true);
    const nextYear = new Date(); nextYear.setFullYear(nextYear.getFullYear() + 1);
    this.form.reset({
      measure_code: 'ALARA-', task_category: '', measure_type: 'shielding', expected_reduction_pct: 25,
      effective_from: utcNowInput(), effective_to: inputToUTC(nextYear.toISOString()).slice(0, 16),
      basis: '', enabled: true,
    });
  }

  edit(measure: ControlMeasure): void {
    this.editing.set(measure); this.formOpen.set(true);
    this.form.setValue({
      measure_code: measure.measure_code, task_category: measure.task_category,
      measure_type: measure.measure_type, expected_reduction_pct: measure.expected_reduction_pct,
      effective_from: measure.effective_from.slice(0, 16), effective_to: measure.effective_to.slice(0, 16),
      basis: measure.basis, enabled: measure.enabled,
    });
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  closeForm(): void { this.formOpen.set(false); this.editing.set(null); }

  save(): void {
    if (this.form.invalid) return;
    const raw = this.form.getRawValue();
    const editing = this.editing();
    const shared = {
      task_category: raw.task_category, measure_type: raw.measure_type,
      expected_reduction_pct: raw.expected_reduction_pct,
      effective_from: inputToUTC(raw.effective_from), effective_to: inputToUTC(raw.effective_to),
      basis: raw.basis,
    };
    const request = editing
      ? this.store.update(editing.id, { ...shared, version: editing.version })
      : this.store.create({ ...shared, measure_code: raw.measure_code, enabled: raw.enabled });
    this.saving.set(true); this.error.set('');
    request.pipe(finalize(() => this.saving.set(false))).subscribe({
      next: () => this.closeForm(),
      error: error => this.error.set(apiErrorMessage(error)),
    });
  }

  toggle(measure: ControlMeasure): void {
    this.error.set('');
    this.store.setStatus(measure.id, !measure.enabled, measure.version).subscribe({
      error: error => this.error.set(apiErrorMessage(error)),
    });
  }

  pickPlan(planId: number): void {
    this.scenarioPlanId.set(planId);
    this.selectedIds.set(new Set());
    this.comparison.set(null);
  }

  referencable(measure: ControlMeasure): boolean { return measure.enabled && !measure.expired; }

  toggleSelection(id: number): void {
    this.selectedIds.update(current => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }

  runScenario(): void {
    const planId = this.scenarioPlanId();
    if (!planId || this.selectedIds().size === 0) return;
    this.scenarioSaving.set(true); this.error.set('');
    this.store.createScenario(planId, [...this.selectedIds()])
      .pipe(finalize(() => this.scenarioSaving.set(false)))
      .subscribe({
        next: scenario => this.comparison.set(scenario.data),
        error: error => this.error.set(apiErrorMessage(error)),
      });
  }

  trackPlan(_: number, plan: WorkPermitPlan): number { return plan.id; }
}
