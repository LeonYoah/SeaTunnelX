/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

'use client';

import {useCallback, useEffect, useState} from 'react';
import {isValidCron} from 'cron-validator';
import {Loader2, Plus, Trash2} from 'lucide-react';
import dynamic from 'next/dynamic';
import {useTranslations} from 'next-intl';
import {toast} from 'sonner';
import services from '@/lib/services';
import type {
  InspectionAutoPolicy,
  InspectionConditionItem,
  InspectionConditionTemplate,
  DiagnosticsClusterOption,
  DiagnosticsTaskOptions,
} from '@/lib/services/diagnostics';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Checkbox} from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Switch} from '@/components/ui/switch';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

interface AutoPolicyConfigPanelProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  clusterOptions: DiagnosticsClusterOption[];
}

const ReUnixCron = dynamic(
  () => import('@sbzen/re-cron').then((mod) => mod.ReUnixCron),
  {ssr: false},
);

const cronEditorStyles = `
  .stx-cron-editor {
    border: 1px solid hsl(var(--border));
    border-radius: calc(var(--radius) - 2px);
    background: hsl(var(--background));
    padding: 0.75rem;
  }

  .stx-cron-editor .c-host {
    font-size: 0.875rem;
  }

  .stx-cron-editor .nav,
  .stx-cron-editor .nav-tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    padding: 0;
    margin: 0 0 0.75rem;
    list-style: none;
    border-bottom: 1px solid hsl(var(--border));
  }

  .stx-cron-editor .nav-item {
    margin: 0;
  }

  .stx-cron-editor .nav-link {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 2rem;
    padding: 0.375rem 0.75rem;
    border: 1px solid transparent;
    border-radius: calc(var(--radius) - 4px);
    background: transparent;
    color: hsl(var(--muted-foreground));
    cursor: pointer;
  }

  .stx-cron-editor .nav-link[aria-selected='true'] {
    border-color: hsl(var(--border));
    background: hsl(var(--muted));
    color: hsl(var(--foreground));
  }

  .stx-cron-editor .c-tab-content {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .stx-cron-editor .form-group,
  .stx-cron-editor .form-inline {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem;
  }

  .stx-cron-editor .form-check {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
  }

  .stx-cron-editor .form-check-input {
    margin: 0;
  }

  .stx-cron-editor .form-check-label {
    line-height: 1.4;
  }

  .stx-cron-editor .form-control {
    min-width: 4.5rem;
    height: 2rem;
    border: 1px solid hsl(var(--border));
    border-radius: calc(var(--radius) - 4px);
    background: hsl(var(--background));
    padding: 0 0.625rem;
    color: hsl(var(--foreground));
  }

  .stx-cron-editor .row {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(4rem, 1fr));
    gap: 0.5rem;
  }

  .stx-cron-editor .row > div {
    min-width: 0;
  }
`;

export function shouldRenderCronExprInput(
  template: InspectionConditionTemplate,
): boolean {
  return Boolean(template.default_cron_expr?.trim());
}

export function applyConditionTextOverride(
  conditions: InspectionConditionItem[],
  templateCode: string,
  field: 'cron_expr_override',
  value: string,
): InspectionConditionItem[] {
  return conditions.map((condition) =>
    condition.template_code === templateCode
      ? {...condition, [field]: value}
      : condition,
  );
}

export function normalizeConditionItemsForSave(
  conditions: InspectionConditionItem[],
): InspectionConditionItem[] {
  return conditions.map((condition) => {
    const cronExprOverride = condition.cron_expr_override?.trim();
    if (!cronExprOverride) {
      const {cron_expr_override, ...rest} = condition;
      return rest;
    }
    return {...condition, cron_expr_override: cronExprOverride};
  });
}

const cronValidationOptions = {
  alias: true,
  allowSevenAsSunday: true,
  allowNthWeekdayOfMonth: true,
};

export function validateCronExpression(value: string): boolean {
  return isValidCron(value.trim(), cronValidationOptions);
}

export function getCronConditionError(
  condition: InspectionConditionItem | undefined,
  template: InspectionConditionTemplate,
): string | null {
  if (!shouldRenderCronExprInput(template)) {
    return null;
  }
  const rawValue = condition?.cron_expr_override ?? '';
  if (!rawValue.trim()) {
    return null;
  }
  return validateCronExpression(rawValue) ? null : 'invalid';
}

export function getCronEditorValue(
  condition: InspectionConditionItem | undefined,
  template: InspectionConditionTemplate,
): string {
  const rawValue = condition?.cron_expr_override ?? '';
  if (rawValue.trim() && validateCronExpression(rawValue)) {
    return rawValue.trim();
  }
  return template.default_cron_expr;
}

export function AutoPolicyConfigPanel({
  open,
  onOpenChange,
  clusterOptions,
}: AutoPolicyConfigPanelProps) {
  const t = useTranslations('diagnosticsCenter.autoPolicies');
  const commonT = useTranslations('common');
  const [policies, setPolicies] = useState<InspectionAutoPolicy[]>([]);
  const [templates, setTemplates] = useState<InspectionConditionTemplate[]>([]);
  const [loading, setLoading] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [editingPolicy, setEditingPolicy] =
    useState<InspectionAutoPolicy | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);

  // Form state
  const [formName, setFormName] = useState('');
  const [formClusterId, setFormClusterId] = useState(0);
  const [formEnabled, setFormEnabled] = useState(true);
  const [formCooldown, setFormCooldown] = useState(30);
  const [formConditions, setFormConditions] = useState<
    InspectionConditionItem[]
  >([]);
  const [formAutoCreateTask, setFormAutoCreateTask] = useState(false);
  const [formAutoStartTask, setFormAutoStartTask] = useState(true);
  const [formTaskOptions, setFormTaskOptions] =
    useState<DiagnosticsTaskOptions>({
      include_thread_dump: true,
      include_jvm_dump: false,
      jvm_dump_min_free_mb: 2048,
    });

  const getCategoryLabel = useCallback(
    (category: string) => {
      switch (category) {
        case 'java_error':
          return t('categories.javaError');
        case 'prometheus':
          return t('categories.prometheus');
        case 'error_rate':
          return t('categories.errorRate');
        case 'node_unhealthy':
          return t('categories.nodeUnhealthy');
        case 'alert_firing':
          return t('categories.alertFiring');
        case 'schedule':
          return t('categories.schedule');
        default:
          return category;
      }
    },
    [t],
  );

  const getPolicyScopeLabel = useCallback(
    (clusterId: number) =>
      clusterId === 0 ? t('scopeGlobal') : t('scopeCluster', {id: clusterId}),
    [t],
  );

  const getAutoBundleModeLabel = useCallback(
    (autoStartTask: boolean) =>
      autoStartTask
        ? t('autoBundleModeCreateAndStart')
        : t('autoBundleModeCreateOnly'),
    [t],
  );

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [policiesResult, templatesResult] = await Promise.all([
        services.diagnostics.listAutoPoliciesSafe({page_size: 100}),
        services.diagnostics.listBuiltinConditionTemplatesSafe(),
      ]);
      if (policiesResult.success && policiesResult.data) {
        setPolicies(policiesResult.data.items || []);
      } else {
        toast.error(policiesResult.error || t('loadPoliciesError'));
      }
      if (templatesResult.success && templatesResult.data) {
        setTemplates(templatesResult.data);
      }
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    if (open) {
      void loadData();
    }
  }, [loadData, open]);

  const openCreateForm = useCallback(() => {
    setEditingPolicy(null);
    setFormName('');
    setFormClusterId(0);
    setFormEnabled(true);
    setFormCooldown(30);
    setFormConditions([]);
    setFormAutoCreateTask(false);
    setFormAutoStartTask(true);
    setFormTaskOptions({
      include_thread_dump: true,
      include_jvm_dump: false,
      jvm_dump_min_free_mb: 2048,
    });
    setFormOpen(true);
  }, []);

  const openEditForm = useCallback((policy: InspectionAutoPolicy) => {
    setEditingPolicy(policy);
    setFormName(policy.name);
    setFormClusterId(policy.cluster_id);
    setFormEnabled(policy.enabled);
    setFormCooldown(policy.cooldown_minutes);
    setFormConditions(policy.conditions || []);
    setFormAutoCreateTask(policy.auto_create_task);
    setFormAutoStartTask(policy.auto_start_task);
    setFormTaskOptions(
      policy.task_options || {
        include_thread_dump: true,
        include_jvm_dump: false,
        jvm_dump_min_free_mb: 2048,
      },
    );
    setFormOpen(true);
  }, []);

  const handleToggleCondition = useCallback(
    (templateCode: string, checked: boolean) => {
      setFormConditions((prev) => {
        if (checked) {
          if (prev.some((c) => c.template_code === templateCode)) {
            return prev.map((c) =>
              c.template_code === templateCode ? {...c, enabled: true} : c,
            );
          }
          return [...prev, {template_code: templateCode, enabled: true}];
        }
        return prev.filter((c) => c.template_code !== templateCode);
      });
    },
    [],
  );

  const handleConditionOverride = useCallback(
    (
      templateCode: string,
      field: 'threshold_override' | 'window_minutes_override',
      value: number | null,
    ) => {
      setFormConditions((prev) =>
        prev.map((c) =>
          c.template_code === templateCode ? {...c, [field]: value} : c,
        ),
      );
    },
    [],
  );

  const handleConditionTextOverride = useCallback(
    (templateCode: string, field: 'cron_expr_override', value: string) => {
      setFormConditions((prev) =>
        applyConditionTextOverride(prev, templateCode, field, value),
      );
    },
    [],
  );

  const handleSave = useCallback(async () => {
    if (!formName.trim()) {
      toast.error(t('nameRequired'));
      return;
    }
    const invalidScheduledCondition = formConditions.find((condition) => {
      if (!condition.enabled) {
        return false;
      }
      const template = templates.find(
        (item) => item.code === condition.template_code,
      );
      if (!template) {
        return false;
      }
      return getCronConditionError(condition, template) !== null;
    });
    if (invalidScheduledCondition) {
      toast.error(t('cronExprInvalid'));
      return;
    }
    setSaving(true);
    try {
      if (editingPolicy) {
        const result = await services.diagnostics.updateAutoPolicySafe(
          editingPolicy.id,
          {
            name: formName,
            enabled: formEnabled,
            conditions: normalizeConditionItemsForSave(formConditions),
            cooldown_minutes: formCooldown,
            auto_create_task: formAutoCreateTask,
            auto_start_task: formAutoStartTask,
            task_options: formAutoCreateTask ? formTaskOptions : undefined,
          },
        );
        if (!result.success) {
          toast.error(result.error || t('updateError'));
          return;
        }
        toast.success(t('updateSuccess'));
      } else {
        const result = await services.diagnostics.createAutoPolicySafe({
          cluster_id: formClusterId,
          name: formName,
          enabled: formEnabled,
          conditions: normalizeConditionItemsForSave(formConditions),
          cooldown_minutes: formCooldown,
          auto_create_task: formAutoCreateTask,
          auto_start_task: formAutoStartTask,
          task_options: formAutoCreateTask ? formTaskOptions : undefined,
        });
        if (!result.success) {
          toast.error(result.error || t('createError'));
          return;
        }
        toast.success(t('createSuccess'));
      }
      setFormOpen(false);
      void loadData();
    } finally {
      setSaving(false);
    }
  }, [
    editingPolicy,
    formAutoCreateTask,
    formAutoStartTask,
    formClusterId,
    formConditions,
    formCooldown,
    formEnabled,
    formName,
    formTaskOptions,
    loadData,
    templates,
    t,
  ]);

  const handleDelete = useCallback(
    async (id: number) => {
      setDeleting(id);
      try {
        const result = await services.diagnostics.deleteAutoPolicySafe(id);
        if (!result.success) {
          toast.error(result.error || t('deleteError'));
          return;
        }
        toast.success(t('deleteSuccess'));
        void loadData();
      } finally {
        setDeleting(null);
      }
    },
    [loadData, t],
  );

  const handleToggleEnabled = useCallback(
    async (policy: InspectionAutoPolicy) => {
      const result = await services.diagnostics.updateAutoPolicySafe(
        policy.id,
        {enabled: !policy.enabled},
      );
      if (!result.success) {
        toast.error(result.error || t('updateError'));
        return;
      }
      void loadData();
    },
    [loadData, t],
  );

  // Group templates by category
  const groupedTemplates = templates.reduce<
    Record<string, InspectionConditionTemplate[]>
  >((acc, tpl) => {
    const key = tpl.category;
    if (!acc[key]) {
      acc[key] = [];
    }
    acc[key].push(tpl);
    return acc;
  }, {});

  return (
    <>
      <style jsx global>
        {cronEditorStyles}
      </style>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='max-w-2xl max-h-[80vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>{t('title')}</DialogTitle>
            <DialogDescription>{t('description')}</DialogDescription>
          </DialogHeader>

          {loading ? (
            <div className='flex items-center justify-center py-8'>
              <Loader2 className='h-6 w-6 animate-spin text-muted-foreground' />
            </div>
          ) : (
            <div className='space-y-4'>
              <div className='flex items-center justify-between'>
                <div className='text-sm text-muted-foreground'>
                  {t('count', {count: policies.length})}
                </div>
                <Button size='sm' onClick={openCreateForm}>
                  <Plus className='mr-2 h-4 w-4' />
                  {t('create')}
                </Button>
              </div>

              {policies.length === 0 ? (
                <div className='flex items-center justify-center rounded-lg border border-dashed p-8 text-sm text-muted-foreground'>
                  {t('empty')}
                </div>
              ) : (
                <div className='space-y-3'>
                  {policies.map((policy) => (
                    <div
                      key={policy.id}
                      className='flex items-center gap-3 rounded-lg border p-4'
                    >
                      <div className='flex-1 min-w-0'>
                        <div className='flex items-center gap-2'>
                          <span className='font-medium truncate'>
                            {policy.name}
                          </span>
                          <Badge variant='outline'>
                            {getPolicyScopeLabel(policy.cluster_id)}
                          </Badge>
                        </div>
                        <div className='mt-1 text-xs text-muted-foreground space-y-1'>
                          <div>
                            {t('conditionsSummary', {
                              count: (policy.conditions || []).length,
                              minutes: policy.cooldown_minutes,
                            })}
                          </div>
                          {policy.auto_create_task ? (
                            <div>
                              {t('autoBundleSummary', {
                                mode: getAutoBundleModeLabel(
                                  policy.auto_start_task,
                                ),
                              })}
                            </div>
                          ) : null}
                        </div>
                      </div>
                      <Switch
                        checked={policy.enabled}
                        onCheckedChange={() => void handleToggleEnabled(policy)}
                      />
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() => openEditForm(policy)}
                      >
                        {commonT('edit')}
                      </Button>
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() => void handleDelete(policy.id)}
                        disabled={deleting === policy.id}
                      >
                        {deleting === policy.id ? (
                          <Loader2 className='h-4 w-4 animate-spin' />
                        ) : (
                          <Trash2 className='h-4 w-4 text-destructive' />
                        )}
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* Create/Edit form dialog */}
      <Dialog open={formOpen} onOpenChange={setFormOpen}>
        <DialogContent className='max-w-2xl max-h-[85vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>
              {editingPolicy ? t('editTitle') : t('createTitle')}
            </DialogTitle>
          </DialogHeader>

          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label>{t('nameLabel')}</Label>
              <Input
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
                placeholder={t('namePlaceholder')}
              />
            </div>

            {!editingPolicy ? (
              <div className='space-y-2'>
                <Label>{t('clusterLabel')}</Label>
                <Select
                  value={String(formClusterId)}
                  onValueChange={(value) =>
                    setFormClusterId(Number.parseInt(value, 10) || 0)
                  }
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t('clusterPlaceholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='0'>{t('globalPolicyOption')}</SelectItem>
                    {clusterOptions.map((cluster) => (
                      <SelectItem
                        key={cluster.cluster_id}
                        value={String(cluster.cluster_id)}
                      >
                        {t('clusterOption', {
                          name: cluster.cluster_name,
                          id: cluster.cluster_id,
                        })}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <div className='text-xs text-muted-foreground'>
                  {t('clusterHint')}
                </div>
              </div>
            ) : null}

            <div className='space-y-2'>
              <Label>{t('cooldownLabel')}</Label>
              <Input
                type='number'
                min={1}
                max={1440}
                value={formCooldown}
                onChange={(e) =>
                  setFormCooldown(Number.parseInt(e.target.value, 10) || 30)
                }
              />
              <div className='text-xs text-muted-foreground'>
                {t('cooldownHint')}
              </div>
            </div>

            <div className='flex items-center gap-2'>
              <Switch checked={formEnabled} onCheckedChange={setFormEnabled} />
              <Label>{t('enabledLabel')}</Label>
            </div>

            <div className='space-y-3 rounded-lg border bg-muted/10 p-4'>
              <div className='flex items-center justify-between'>
                <div>
                  <div className='font-medium'>{t('autoBundleTitle')}</div>
                  <div className='text-xs text-muted-foreground'>
                    {t('autoBundleHint')}
                  </div>
                </div>
                <Switch
                  checked={formAutoCreateTask}
                  onCheckedChange={setFormAutoCreateTask}
                />
              </div>

              {formAutoCreateTask ? (
                <div className='mt-3 grid gap-4 md:grid-cols-2'>
                  <div className='flex items-center justify-between rounded-lg border bg-background p-3'>
                    <div>
                      <div className='font-medium'>
                        {t('includeThreadDump')}
                      </div>
                      <div className='text-xs text-muted-foreground'>
                        {t('includeThreadDumpHint')}
                      </div>
                    </div>
                    <Switch
                      checked={formTaskOptions.include_thread_dump}
                      onCheckedChange={(checked) =>
                        setFormTaskOptions(
                          (current: DiagnosticsTaskOptions) => ({
                            ...current,
                            include_thread_dump: checked,
                          }),
                        )
                      }
                    />
                  </div>

                  <div className='flex items-center justify-between rounded-lg border bg-background p-3'>
                    <div>
                      <div className='font-medium'>{t('includeJVMDump')}</div>
                      <div className='text-xs text-muted-foreground'>
                        {t('includeJVMDumpHint')}
                      </div>
                    </div>
                    <Switch
                      checked={formTaskOptions.include_jvm_dump}
                      onCheckedChange={(checked) =>
                        setFormTaskOptions(
                          (current: DiagnosticsTaskOptions) => ({
                            ...current,
                            include_jvm_dump: checked,
                          }),
                        )
                      }
                    />
                  </div>

                  <div className='space-y-2'>
                    <Label htmlFor='auto-policy-jvm-space'>
                      {t('jvmMinFreeMBLabel')}
                    </Label>
                    <Input
                      id='auto-policy-jvm-space'
                      type='number'
                      min={256}
                      step={256}
                      value={formTaskOptions.jvm_dump_min_free_mb ?? 2048}
                      onChange={(event) =>
                        setFormTaskOptions(
                          (current: DiagnosticsTaskOptions) => ({
                            ...current,
                            jvm_dump_min_free_mb:
                              Number.parseInt(event.target.value, 10) || 2048,
                          }),
                        )
                      }
                      disabled={!formTaskOptions.include_jvm_dump}
                    />
                    <div className='text-xs text-muted-foreground'>
                      {t('jvmMinFreeMBHint')}
                    </div>
                  </div>

                  <div className='flex items-center gap-2 md:col-span-2'>
                    <Switch
                      checked={formAutoStartTask}
                      onCheckedChange={setFormAutoStartTask}
                    />
                    <div className='text-sm text-muted-foreground'>
                      {t('autoStartTaskLabel')}
                    </div>
                  </div>
                </div>
              ) : null}
            </div>

            <div className='space-y-3'>
              <Label>{t('conditionsLabel')}</Label>
              {Object.entries(groupedTemplates).map(
                ([category, categoryTemplates]) => (
                  <div
                    key={category}
                    className='rounded-lg border p-3 space-y-2'
                  >
                    <div className='text-sm font-medium'>
                      {getCategoryLabel(category)}
                    </div>
                    {categoryTemplates.map((tpl) => {
                      const isChecked = formConditions.some(
                        (c) => c.template_code === tpl.code,
                      );
                      const condition = formConditions.find(
                        (c) => c.template_code === tpl.code,
                      );
                      const cronError = getCronConditionError(condition, tpl);
                      return (
                        <div
                          key={tpl.code}
                          className='space-y-2 rounded-md border bg-muted/20 p-2'
                        >
                          <div className='flex items-center gap-2'>
                            <Checkbox
                              checked={isChecked}
                              onCheckedChange={(checked) =>
                                handleToggleCondition(
                                  tpl.code,
                                  checked === true,
                                )
                              }
                            />
                            <div className='flex-1'>
                              <div className='text-sm font-medium'>
                                {tpl.name}
                              </div>
                              <div className='text-xs text-muted-foreground'>
                                {tpl.description}
                              </div>
                            </div>
                          </div>
                          {isChecked && !tpl.immediate_on_match ? (
                            <div className='ml-6 grid grid-cols-2 gap-2'>
                              {shouldRenderCronExprInput(tpl) ? (
                                <div className='space-y-1 col-span-2'>
                                  <Label className='text-xs'>
                                    {t('cronExprLabel')}
                                  </Label>
                                  <div className='text-xs text-muted-foreground'>
                                    {t('cronEditorLabel')}
                                  </div>
                                  <div className='stx-cron-editor'>
                                    <ReUnixCron
                                      value={getCronEditorValue(condition, tpl)}
                                      onChange={(value: string) =>
                                        handleConditionTextOverride(
                                          tpl.code,
                                          'cron_expr_override',
                                          value,
                                        )
                                      }
                                    />
                                  </div>
                                  <Input
                                    type='text'
                                    className='h-8 text-xs font-mono'
                                    placeholder={tpl.default_cron_expr}
                                    value={condition?.cron_expr_override ?? ''}
                                    aria-invalid={cronError ? true : undefined}
                                    onChange={(e) =>
                                      handleConditionTextOverride(
                                        tpl.code,
                                        'cron_expr_override',
                                        e.target.value,
                                      )
                                    }
                                  />
                                  <div className='text-xs text-muted-foreground'>
                                    {t('cronExprHint', {
                                      defaultExpr: tpl.default_cron_expr,
                                    })}
                                  </div>
                                  {cronError ? (
                                    <div className='text-xs text-destructive'>
                                      {t('cronExprInvalid')}
                                    </div>
                                  ) : null}
                                </div>
                              ) : null}
                              {tpl.default_threshold > 0 ? (
                                <div className='space-y-1'>
                                  <Label className='text-xs'>
                                    {t('thresholdLabel', {
                                      value: tpl.default_threshold,
                                    })}
                                  </Label>
                                  <Input
                                    type='number'
                                    className='h-8 text-xs'
                                    placeholder={String(tpl.default_threshold)}
                                    value={condition?.threshold_override ?? ''}
                                    onChange={(e) =>
                                      handleConditionOverride(
                                        tpl.code,
                                        'threshold_override',
                                        e.target.value
                                          ? Number(e.target.value)
                                          : null,
                                      )
                                    }
                                  />
                                </div>
                              ) : null}
                              {tpl.default_window_minutes > 0 ? (
                                <div className='space-y-1'>
                                  <Label className='text-xs'>
                                    {t('windowLabel', {
                                      minutes: tpl.default_window_minutes,
                                    })}
                                  </Label>
                                  <Input
                                    type='number'
                                    className='h-8 text-xs'
                                    placeholder={String(
                                      tpl.default_window_minutes,
                                    )}
                                    value={
                                      condition?.window_minutes_override ?? ''
                                    }
                                    onChange={(e) =>
                                      handleConditionOverride(
                                        tpl.code,
                                        'window_minutes_override',
                                        e.target.value
                                          ? Number(e.target.value)
                                          : null,
                                      )
                                    }
                                  />
                                </div>
                              ) : null}
                            </div>
                          ) : null}
                        </div>
                      );
                    })}
                  </div>
                ),
              )}
              {templates.length === 0 ? (
                <div className='text-sm text-muted-foreground'>
                  {t('noTemplates')}
                </div>
              ) : null}
            </div>
          </div>

          <DialogFooter>
            <Button variant='outline' onClick={() => setFormOpen(false)}>
              {commonT('cancel')}
            </Button>
            <Button onClick={() => void handleSave()} disabled={saving}>
              {saving ? (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              ) : null}
              {editingPolicy ? t('saveEdit') : t('createConfirm')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
