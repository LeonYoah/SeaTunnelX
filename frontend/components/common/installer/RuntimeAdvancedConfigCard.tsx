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

import {useTranslations} from 'next-intl';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {Label} from '@/components/ui/label';
import {Input} from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Checkbox} from '@/components/ui/checkbox';
import {Settings2} from 'lucide-react';
import type {
  RuntimeEngineConfig,
  SeaTunnelVersionCapabilities,
  JobScheduleStrategy,
  SlotAllocationStrategy,
  JobLogMode,
} from '@/lib/services/installer/types';

interface RuntimeAdvancedConfigCardProps {
  version?: string;
  capabilities?: SeaTunnelVersionCapabilities | null;
  runtime: RuntimeEngineConfig;
  onChange: (updates: Partial<RuntimeEngineConfig>) => void;
}

export function RuntimeAdvancedConfigCard({
  version,
  capabilities,
  runtime,
  onChange,
}: RuntimeAdvancedConfigCardProps) {
  const t = useTranslations();

  const showSlotSettings =
    capabilities?.supports_dynamic_slot || capabilities?.supports_slot_num;
  const showHistorySettings =
    capabilities?.supports_history_job_expire_minutes ||
    capabilities?.supports_scheduled_deletion_enable;
  const showJobLogSettings = capabilities?.supports_job_log_mode;
  const slotAllocationDescription =
    runtime.slot_allocation_strategy === 'SYSTEM_LOAD'
      ? t('installer.runtimeAdvanced.slotAllocationSystemLoadDesc')
      : runtime.slot_allocation_strategy === 'SLOT_RATIO'
        ? t('installer.runtimeAdvanced.slotAllocationSlotRatioDesc')
        : t('installer.runtimeAdvanced.slotAllocationRandomDesc');

  return (
    <Card>
      <CardHeader className='pb-3'>
        <CardTitle className='text-base flex items-center gap-2'>
          <Settings2 className='h-4 w-4' />
          {t('installer.runtimeAdvanced.title')}
        </CardTitle>
        <CardDescription>
          {t('installer.runtimeAdvanced.description')}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        {!version ? (
          <p className='text-sm text-muted-foreground'>
            {t('installer.runtimeAdvanced.selectVersionHint')}
          </p>
        ) : (
          <>
            {showSlotSettings && (
              <div className='space-y-4 rounded-lg border p-4'>
                <div className='space-y-1'>
                  <h4 className='text-sm font-medium'>
                    {t('installer.runtimeAdvanced.slotSectionTitle')}
                  </h4>
                  <p className='text-xs text-muted-foreground'>
                    {t('installer.runtimeAdvanced.slotSectionDescription')}
                  </p>
                </div>

                <div className='space-y-2'>
                  <Label>{t('installer.runtimeAdvanced.slotMode')}</Label>
                  <Select
                    value={runtime.dynamic_slot ? 'dynamic' : 'static'}
                    onValueChange={(value) =>
                      onChange({dynamic_slot: value === 'dynamic'})
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='dynamic'>
                        {t('installer.runtimeAdvanced.dynamicSlot')}
                      </SelectItem>
                      <SelectItem value='static'>
                        {t('installer.runtimeAdvanced.staticSlot')}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                {capabilities?.supports_slot_allocation_strategy ? (
                  <div className='space-y-2'>
                    <Label>
                      {t('installer.runtimeAdvanced.slotAllocationStrategy')}
                    </Label>
                    <Select
                      value={runtime.slot_allocation_strategy}
                      onValueChange={(value: SlotAllocationStrategy) =>
                        onChange({slot_allocation_strategy: value})
                      }
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value='RANDOM'>
                          {t('installer.runtimeAdvanced.slotAllocationRandom')}
                        </SelectItem>
                        <SelectItem value='SYSTEM_LOAD'>
                          {t(
                            'installer.runtimeAdvanced.slotAllocationSystemLoad',
                          )}
                        </SelectItem>
                        <SelectItem value='SLOT_RATIO'>
                          {t(
                            'installer.runtimeAdvanced.slotAllocationSlotRatio',
                          )}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <p className='text-xs text-muted-foreground'>
                      {t('installer.runtimeAdvanced.slotAllocationHint')}
                    </p>
                    <div className='rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground'>
                      {slotAllocationDescription}
                    </div>
                  </div>
                ) : (
                  <p className='text-xs text-muted-foreground'>
                    {t('installer.runtimeAdvanced.slotAllocationUnsupported')}
                  </p>
                )}

                {runtime.dynamic_slot ? (
                  <p className='text-xs text-muted-foreground'>
                    {t('installer.runtimeAdvanced.dynamicSlotHint')}
                  </p>
                ) : (
                  <div className='grid gap-4 md:grid-cols-2'>
                    {capabilities?.supports_slot_num && (
                      <div className='space-y-2'>
                        <Label>{t('installer.runtimeAdvanced.slotNum')}</Label>
                        <Input
                          type='number'
                          value={runtime.slot_num}
                          onChange={(event) =>
                            onChange({
                              slot_num: Math.max(
                                1,
                                Number.parseInt(event.target.value, 10) || 1,
                              ),
                            })
                          }
                          min={1}
                          step={1}
                        />
                        <p className='text-xs text-muted-foreground'>
                          {t('installer.runtimeAdvanced.slotNumHint')}
                        </p>
                      </div>
                    )}

                    {capabilities?.supports_job_schedule_strategy ? (
                      <div className='space-y-2'>
                        <Label>
                          {t('installer.runtimeAdvanced.jobScheduleStrategy')}
                        </Label>
                        <Select
                          value={runtime.job_schedule_strategy}
                          onValueChange={(value: JobScheduleStrategy) =>
                            onChange({job_schedule_strategy: value})
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value='REJECT'>
                              {t('installer.runtimeAdvanced.jobScheduleReject')}
                            </SelectItem>
                            <SelectItem value='WAIT'>
                              {t('installer.runtimeAdvanced.jobScheduleWait')}
                            </SelectItem>
                          </SelectContent>
                        </Select>
                        <p className='text-xs text-muted-foreground'>
                          {t('installer.runtimeAdvanced.jobScheduleHint')}
                        </p>
                      </div>
                    ) : (
                      <div className='space-y-2'>
                        <Label>
                          {t('installer.runtimeAdvanced.jobScheduleStrategy')}
                        </Label>
                        <div className='rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground'>
                          {t(
                            'installer.runtimeAdvanced.jobScheduleUnsupported',
                          )}
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {showHistorySettings ? (
              <div className='space-y-4 rounded-lg border p-4'>
                <div className='space-y-1'>
                  <h4 className='text-sm font-medium'>
                    {t('installer.runtimeAdvanced.historySectionTitle')}
                  </h4>
                  <p className='text-xs text-muted-foreground'>
                    {t('installer.runtimeAdvanced.historySectionDescription')}
                  </p>
                </div>

                {capabilities?.supports_history_job_expire_minutes ? (
                  <div className='space-y-2'>
                    <Label>
                      {t('installer.runtimeAdvanced.historyJobExpireMinutes')}
                    </Label>
                    <Input
                      type='number'
                      value={runtime.history_job_expire_minutes}
                      onChange={(event) =>
                        onChange({
                          history_job_expire_minutes: Math.max(
                            1,
                            Number.parseInt(event.target.value, 10) || 1,
                          ),
                        })
                      }
                      min={1}
                      step={1}
                    />
                    <p className='text-xs text-muted-foreground'>
                      {t('installer.runtimeAdvanced.historyJobExpireHint')}
                    </p>
                  </div>
                ) : (
                  <p className='text-xs text-muted-foreground'>
                    {t('installer.runtimeAdvanced.historyUnsupported')}
                  </p>
                )}

                {capabilities?.supports_scheduled_deletion_enable ? (
                  <label className='flex items-start gap-3 rounded-md border p-3'>
                    <Checkbox
                      checked={runtime.scheduled_deletion_enable}
                      onCheckedChange={(checked) =>
                        onChange({scheduled_deletion_enable: checked === true})
                      }
                    />
                    <div className='space-y-1'>
                      <div className='text-sm font-medium'>
                        {t('installer.runtimeAdvanced.scheduledDeletionEnable')}
                      </div>
                      <p className='text-xs text-muted-foreground'>
                        {t('installer.runtimeAdvanced.scheduledDeletionHint')}
                      </p>
                    </div>
                  </label>
                ) : (
                  <p className='text-xs text-muted-foreground'>
                    {t(
                      'installer.runtimeAdvanced.scheduledDeletionUnsupported',
                    )}
                  </p>
                )}
              </div>
            ) : (
              <p className='text-xs text-muted-foreground'>
                {t('installer.runtimeAdvanced.historyUnsupported')}
              </p>
            )}

            {showJobLogSettings ? (
              <div className='space-y-4 rounded-lg border p-4'>
                <div className='space-y-1'>
                  <h4 className='text-sm font-medium'>
                    {t('installer.jobLogMode.title')}
                  </h4>
                  <p className='text-xs text-muted-foreground'>
                    {t('installer.jobLogMode.description')}
                  </p>
                </div>
                <div className='space-y-2'>
                  <Label>{t('installer.jobLogMode.mode')}</Label>
                  <Select
                    value={runtime.job_log_mode}
                    onValueChange={(value: JobLogMode) =>
                      onChange({job_log_mode: value})
                    }
                  >
                    <SelectTrigger data-testid='install-runtime-log-mode'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='mixed'>
                        {t('installer.jobLogMode.mixed')}
                      </SelectItem>
                      <SelectItem value='per_job'>
                        {t('installer.jobLogMode.perJob')}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <p className='text-xs text-muted-foreground'>
                    {runtime.job_log_mode === 'per_job'
                      ? t('installer.jobLogMode.perJobHint')
                      : t('installer.jobLogMode.mixedHint')}
                  </p>
                </div>
              </div>
            ) : null}
          </>
        )}
      </CardContent>
    </Card>
  );
}
