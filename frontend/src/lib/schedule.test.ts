import { describe, expect, it } from 'vitest'
import { EVERY_DAY, WEEKDAYS, WEEKENDS, daysLabel, daysLabelFromNow, describeSchedule, intervalLabel, nextRunLabel } from './schedule'

describe('schedule labels', () => {
  it('names intervals', () => {
    expect(intervalLabel(15)).toBe('Every 15 minutes')
    expect(intervalLabel(60)).toBe('Every hour')
    expect(intervalLabel(360)).toBe('Every 6 hours')
    expect(intervalLabel(90)).toBe('Every 90 minutes')
    expect(intervalLabel(1440)).toBe('Once a day')
  })

  it('names day sets, Monday first', () => {
    expect(daysLabel(EVERY_DAY)).toBe('every day')
    expect(daysLabel(WEEKDAYS)).toBe('Mon–Fri')
    expect(daysLabel(WEEKENDS)).toBe('weekends')
    expect(daysLabel((1 << 0) | (1 << 1) | (1 << 3))).toBe('Mon, Wed, Sun')
  })

  it('describes a schedule', () => {
    expect(describeSchedule({ interval_minutes: 120, schedule_start: '07:30', schedule_days: WEEKDAYS })).toBe(
      'Every 2 hours from 07:30, Mon–Fri',
    )
    expect(describeSchedule({ interval_minutes: 1440, schedule_start: '02:00', schedule_days: EVERY_DAY })).toBe(
      'Once a day at 02:00, every day',
    )
  })

  it('says when the next run is', () => {
    const now = 1_700_000_000_000
    expect(nextRunLabel(null, now)).toBe('within a minute')
    expect(nextRunLabel(now / 1000 + 30, now)).toBe('within a minute')
    expect(nextRunLabel(now / 1000 + 12 * 60, now)).toBe('in 12 min')
    expect(nextRunLabel(now / 1000 + 3 * 3600, now)).toBe('in 3 h')
  })

  it('says when the queue finishes', () => {
    expect(daysLabelFromNow(0)).toBe('today')
    expect(daysLabelFromNow(1)).toBe('by tomorrow')
    expect(daysLabelFromNow(4)).toBe('in about 4 days')
    expect(daysLabelFromNow(-1)).toMatch(/not until/)
  })
})
