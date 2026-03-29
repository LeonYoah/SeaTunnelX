'use client';

import {useEffect, useMemo, useState, type Dispatch, type ReactNode, type SetStateAction} from 'react';

import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {Tooltip, TooltipContent, TooltipTrigger} from '@/components/ui/tooltip';

import {
  cronToText,
  expressionFromParts,
  getNextRuns,
  parseExpressionToParts,
  type CronTranslator,
} from '@/components/common/sync/task-schedule-cron';

export interface CronPresetOption {
  key: string;
  expr: string;
}

export interface CronExpressionEditorLabels {
  cronExpression: string;
  timezone: string;
  cronFiveField: string;
  cronTimezonePlaceholder: string;
  cronExpressionPlaceholder: string;
  cronMinute: string;
  cronHour: string;
  cronDayOfMonth: string;
  cronMonth: string;
  cronDayOfWeek: string;
  schedulePreview: string;
  nextRuns: string;
  invalidCronExpression: string;
}

function formatDate(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}/${date.getMonth() + 1}/${date.getDate()} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

function BuilderFields({
  parts,
  setParts,
  labels,
}: {
  parts: Record<string, string>;
  setParts: Dispatch<SetStateAction<Record<string, string>>>;
  labels: CronExpressionEditorLabels;
}) {
  return (
    <div className='grid gap-2'>
      {(['minute', 'hour', 'dayOfMonth', 'month', 'dayOfWeek'] as const).map((key) => (
        <div key={key} className='space-y-1.5'>
          <Label className='text-[11px] text-muted-foreground'>{labels[`cron${key.charAt(0).toUpperCase()}${key.slice(1)}` as keyof CronExpressionEditorLabels] as string}</Label>
          <Input
            value={parts[key]}
            onChange={(e) => setParts((prev) => ({...prev, [key]: e.target.value}))}
            className='h-9 text-xs font-mono'
          />
        </div>
      ))}
    </div>
  );
}

export function CronExpressionEditor({
  expression,
  onExpressionChange,
  timezone,
  onTimezoneChange,
  translator,
  labels,
  presets,
  renderPresetLabel,
  footer,
  helper,
}: {
  expression: string;
  onExpressionChange: (value: string) => void;
  timezone?: string;
  onTimezoneChange?: (value: string) => void;
  translator: CronTranslator;
  labels: CronExpressionEditorLabels;
  presets: CronPresetOption[];
  renderPresetLabel: (key: string) => string;
  footer?: ReactNode;
  helper?: ReactNode;
}) {
  const [parts, setParts] = useState<Record<string, string>>(() => {
    try {
      return parseExpressionToParts(expression || '0 0 * * *');
    } catch {
      return {minute: '0', hour: '0', dayOfMonth: '*', month: '*', dayOfWeek: '*'};
    }
  });

  useEffect(() => {
    try {
      setParts(parseExpressionToParts(expression || '0 0 * * *'));
    } catch {
      // ignore invalid external expression and keep local builder state
    }
  }, [expression]);

  const derivedExpression = useMemo(() => expressionFromParts(parts), [parts]);
  const previewState = useMemo(() => {
    try {
      return {
        ok: true,
        text: cronToText(expression || derivedExpression, translator),
        nextRuns: getNextRuns(expression || derivedExpression, 5),
      } as const;
    } catch (error) {
      return {
        ok: false,
        error: error instanceof Error ? error.message : labels.invalidCronExpression,
      } as const;
    }
  }, [derivedExpression, expression, labels.invalidCronExpression, translator]);

  return (
    <div className='space-y-3'>
      {onTimezoneChange ? (
        <div className='space-y-1.5'>
          <Label className='text-xs'>{labels.timezone}</Label>
          <Input
            value={timezone || ''}
            onChange={(e) => onTimezoneChange(e.target.value)}
            className='h-9 text-xs'
            placeholder={labels.cronTimezonePlaceholder}
          />
        </div>
      ) : null}

      <div className='space-y-1.5'>
        <div className='flex items-center justify-between gap-2'>
          <Label className='text-xs'>{labels.cronExpression}</Label>
          <Badge variant='outline' className='rounded-sm text-[10px]'>
            {labels.cronFiveField}
          </Badge>
        </div>
        <Input
          value={expression}
          onChange={(e) => {
            const nextExpr = e.target.value;
            try {
              setParts(parseExpressionToParts(nextExpr));
            } catch {
              onExpressionChange(nextExpr);
              return;
            }
            onExpressionChange(nextExpr);
          }}
          className='h-9 text-xs font-mono'
          placeholder={labels.cronExpressionPlaceholder}
        />
        {helper ? <div className='text-xs text-muted-foreground'>{helper}</div> : null}
      </div>

      <BuilderFields
        parts={parts}
        setParts={(next) => {
          const resolved = typeof next === 'function' ? next(parts) : next;
          setParts(resolved);
          onExpressionChange(expressionFromParts(resolved));
        }}
        labels={labels}
      />

      <div className='flex flex-wrap gap-2'>
        {presets.map((preset) => (
          <Button
            key={preset.key}
            type='button'
            variant='secondary'
            className='h-8 rounded-md px-2 text-[11px]'
            onClick={() => {
              setParts(parseExpressionToParts(preset.expr));
              onExpressionChange(preset.expr);
            }}
          >
            {renderPresetLabel(preset.key)}
          </Button>
        ))}
      </div>

      <div className='rounded-lg border border-border/50 bg-muted/10 p-3'>
        <div className='mb-2 flex items-center justify-between gap-2'>
          <Label className='text-xs'>{labels.schedulePreview}</Label>
          {footer}
        </div>
        {previewState.ok ? (
          <div className='space-y-3 text-xs'>
            <div className='rounded-md border border-border/50 bg-background/70 p-2'>
              <div className='font-mono text-[11px]'>{expression || derivedExpression}</div>
              <div className='mt-1 text-muted-foreground'>{previewState.text}</div>
            </div>
            <div>
              <div className='mb-1 text-[11px] text-muted-foreground'>{labels.nextRuns}</div>
              <div className='space-y-1'>
                {previewState.nextRuns.map((run, index) => (
                  <div key={`${run.toISOString()}-${index}`} className='rounded-md border border-border/50 bg-background/60 px-2 py-1.5 font-mono text-[11px]'>
                    {formatDate(run)}
                  </div>
                ))}
              </div>
            </div>
          </div>
        ) : (
          <div className='text-xs text-destructive'>{previewState.error}</div>
        )}
      </div>
    </div>
  );
}
