import {describe, expect, it} from 'vitest';

import {cronToText, getNextRuns, parseCron} from './task-schedule-cron';

const t = (key: string, values?: Record<string, string | number>) => {
  switch (key) {
    case 'cronEveryMinute': return 'Every minute';
    case 'cronEveryNMinutes': return `Every ${values?.count} minutes`;
    case 'cronEveryNMinutesDuringHour': return `Every ${values?.count} minutes during hour ${values?.hour}:00`;
    case 'cronEveryHour': return 'Every hour';
    case 'cronEveryDayAtMidnight': return 'Every day at 00:00';
    case 'cronWeekdaysAtNine': return 'Weekdays at 09:00';
    case 'cronWeekdaysAtEightThirty': return 'Weekdays at 08:30';
    case 'cronWeeklySunday': return 'Every Sunday at 00:00';
    case 'cronMonthlyFirstDayNine': return 'On the 1st day of every month at 09:00';
    case 'cronQuarterlyFirstDay': return 'Quarterly on day 1 at 00:00';
    case 'cronAtTime': return `At ${values?.hour}:${values?.minute}`;
    case 'cronRunsOnExpr': return `Runs on ${values?.expr}`;
    case 'invalidCronExpression': return 'Invalid cron expression';
    default: return key;
  }
};

describe('task schedule cron', () => {
  it('describes */10 1 * * * without NaN', () => {
    expect(cronToText('*/10 1 * * *', t)).toBe('Every 10 minutes during hour 01:00');
  });

  it('describes fixed daily time', () => {
    expect(cronToText('30 8 * * *', t)).toBe('At 08:30');
  });

  it('supports named weekdays', () => {
    const parsed = parseCron('0 9 * * MON-FRI');
    expect(parsed.expanded.dayOfWeek).toEqual([1, 2, 3, 4, 5]);
  });

  it('returns next runs for complex expression', () => {
    const base = new Date('2026-03-29T00:55:00+08:00');
    const runs = getNextRuns('*/10 1 * * *', 3, base);
    expect(runs).toHaveLength(3);
    expect(runs[0].getHours()).toBe(1);
    expect(runs[0].getMinutes()).toBe(0);
    expect(runs[1].getMinutes()).toBe(10);
    expect(runs[2].getMinutes()).toBe(20);
  });


  it('describes quarterly expression with preset text', () => {
    expect(cronToText('0 0 1 */3 *', t)).toBe('Quarterly on day 1 at 00:00');
  });

  it('supports named months', () => {
    const parsed = parseCron('0 9 1 JAN,MAR,DEC *');
    expect(parsed.expanded.month).toEqual([1, 3, 12]);
  });

  it('treats 7 as sunday in day of week', () => {
    const parsed = parseCron('0 0 * * 7');
    expect(parsed.expanded.dayOfWeek).toEqual([0]);
  });

  it('handles dom and dow together with OR semantics', () => {
    const runs = getNextRuns('0 9 1 * MON', 4, new Date('2026-03-30T08:00:00+08:00'));
    expect(runs.map((run) => run.toISOString())).toEqual([
      '2026-03-30T01:00:00.000Z',
      '2026-04-01T01:00:00.000Z',
      '2026-04-06T01:00:00.000Z',
      '2026-04-13T01:00:00.000Z',
    ]);
  });

  it('supports list range and step combinations', () => {
    const parsed = parseCron('5,10-12,20-30/5 8 * * *');
    expect(parsed.expanded.minute).toEqual([5, 10, 11, 12, 20, 25, 30]);
  });

  it('supports named month ranges and weekday ranges', () => {
    const parsed = parseCron('0 18 * JAN-MAR MON-WED');
    expect(parsed.expanded.month).toEqual([1, 2, 3]);
    expect(parsed.expanded.dayOfWeek).toEqual([1, 2, 3]);
  });

  it('rejects out-of-bounds values', () => {
    expect(() => parseCron('61 0 * * *')).toThrow(/Out of bounds/);
  });


  it('rejects invalid 6-field cron', () => {
    expect(() => parseCron('0 0 1 * * *')).toThrow(/5-field/);
  });
});
