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

import {useMemo} from 'react';

import {cn} from '@/lib/utils';
import {useTranslations} from 'next-intl';

import {Badge} from '@/components/ui/badge';
import {Label} from '@/components/ui/label';
import {Switch} from '@/components/ui/switch';
import {Tooltip, TooltipContent, TooltipTrigger} from '@/components/ui/tooltip';

import {
  CronExpressionEditor,
  type CronPresetOption,
} from '@/components/common/schedule/CronExpressionEditor';


export interface TaskScheduleValue {
  enabled: boolean;
  cron_expr: string;
  timezone: string;
}

export function TaskScheduleSidebarPanel({
  value,
  lastTriggeredAt,
  nextTriggeredAt,
  onChange,
  className,
}: {
  value: TaskScheduleValue;
  lastTriggeredAt?: string;
  nextTriggeredAt?: string;
  onChange: (value: TaskScheduleValue) => void;
  className?: string;
}) {
  const t = useTranslations('workbenchStudio');
  const presets = useMemo<CronPresetOption[]>(() => [
    {key: 'cronPresetEveryDayMidnight', expr: '0 0 * * *'},
    {key: 'cronPresetEveryHour', expr: '0 * * * *'},
    {key: 'cronPresetWeekdaysNine', expr: '0 9 * * MON-FRI'},
    {key: 'cronPresetEveryFifteenMinutes', expr: '*/15 * * * *'},
    {key: 'cronPresetEveryThirtyMinutes', expr: '*/30 * * * *'},
    {key: 'cronPresetWeekdaysEightThirty', expr: '30 8 * * MON-FRI'},
    {key: 'cronPresetWeeklySundayMidnight', expr: '0 0 * * SUN'},
    {key: 'cronPresetMonthlyFirstNine', expr: '0 9 1 * *'},
    {key: 'cronPresetQuarterlyFirst', expr: '0 0 1 */3 *'},
  ], []);

  const timezoneValue = value.timezone || 'Asia/Shanghai';

  const formatScheduleTime = (value?: string) => {
    if (!value) {
      return t('scheduleNoTriggerYet');
    }
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) {
      return value;
    }
    return new Intl.DateTimeFormat(undefined, {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    }).format(parsed);
  };


  return (
    <div className={cn('min-w-0 space-y-4', className)}>
      <div className='rounded-lg border border-border/50 bg-muted/10 p-4 md:p-5'>
        <div className='mb-3 flex items-center justify-between gap-2'>
          <div>
            <Label className='text-xs font-medium uppercase tracking-wide text-muted-foreground'>
              {t('taskSchedule')}
            </Label>
            <p className='mt-1 text-[11px] leading-5 text-muted-foreground'>
              {t('taskScheduleHint')}
            </p>
          </div>
          <Switch
            checked={value.enabled}
            onCheckedChange={(checked) => onChange({...value, enabled: checked, cron_expr: value.cron_expr, timezone: timezoneValue})}
          />
        </div>


        <div className='mb-4 grid gap-3 rounded-md border border-border/50 bg-background/60 p-3 md:grid-cols-2'>
          <div className='space-y-1'>
            <div className='text-[10px] uppercase tracking-wide text-muted-foreground'>
              {t('scheduleLastTriggeredAt')}
            </div>
            <div className='text-xs font-medium text-foreground'>
              {formatScheduleTime(lastTriggeredAt)}
            </div>
          </div>
          <div className='space-y-1'>
            <div className='text-[10px] uppercase tracking-wide text-muted-foreground'>
              {t('scheduleNextTriggeredAt')}
            </div>
            <div className='text-xs font-medium text-foreground'>
              {value.enabled ? formatScheduleTime(nextTriggeredAt) : t('scheduleDisabled')}
            </div>
          </div>
        </div>

        <CronExpressionEditor
          expression={value.cron_expr}
          onExpressionChange={(cron_expr) => onChange({...value, cron_expr, timezone: timezoneValue})}
          timezone={timezoneValue}
          onTimezoneChange={(timezone) => onChange({...value, timezone, cron_expr: value.cron_expr})}
          translator={(key, values) => t(key, values as never)}
          labels={{
            cronExpression: t('cronExpression'),
            timezone: t('timezone'),
            cronFiveField: t('cronFiveField'),
            cronTimezonePlaceholder: t('cronTimezonePlaceholder'),
            cronExpressionPlaceholder: t('cronExpressionPlaceholder'),
            cronMinute: t('cronMinute'),
            cronHour: t('cronHour'),
            cronDayOfMonth: t('cronDayOfMonth'),
            cronMonth: t('cronMonth'),
            cronDayOfWeek: t('cronDayOfWeek'),
            schedulePreview: t('schedulePreview'),
            nextRuns: t('nextRuns'),
            invalidCronExpression: t('invalidCronExpression'),
          }}
          presets={presets}
          renderPresetLabel={(key) => t(key)}
          footer={
            <Tooltip>
              <TooltipTrigger asChild>
                <Badge variant='outline' className='cursor-help rounded-sm text-[10px]'>
                  {t('savedVersionOnly')}
                </Badge>
              </TooltipTrigger>
              <TooltipContent className='max-w-[320px] text-xs leading-5'>
                {t('scheduleSavedVersionHint')}
              </TooltipContent>
            </Tooltip>
          }
        />
      </div>
    </div>
  );
}
