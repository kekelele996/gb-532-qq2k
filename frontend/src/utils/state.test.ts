import { describe, expect, it } from 'vitest'
import { RUN_STATE_LABEL, RUN_TRANSITIONS } from '../types/enums/run-state'
import { GAP_SEVERITY_LABEL, GAP_STATE_LABEL } from '../types/enums/gap-severity'
import { RESURVEY_TASK_STATE_LABEL, RESURVEY_TASK_TRANSITIONS } from '../types/enums/resurvey-task'

describe('shared workflow enumerations',()=>{
  it('prevents imported runs from skipping quality and processing',()=>{expect(RUN_TRANSITIONS.imported).toEqual(['quality_checked','rejected']);expect(RUN_TRANSITIONS.imported).not.toContain('processed')})
  it('provides operator-facing labels for every state and severity',()=>{expect(Object.keys(RUN_STATE_LABEL)).toHaveLength(6);expect(Object.keys(GAP_STATE_LABEL)).toHaveLength(7);expect(Object.keys(GAP_SEVERITY_LABEL)).toHaveLength(3)})
  it('drives resurvey tasks pending -> running -> awaiting_review with terminal exits',()=>{
    expect(Object.keys(RESURVEY_TASK_STATE_LABEL)).toHaveLength(5)
    expect(RESURVEY_TASK_TRANSITIONS.pending).toEqual(['running','canceled'])
    expect(RESURVEY_TASK_TRANSITIONS.running).toEqual(['awaiting_review','canceled'])
    expect(RESURVEY_TASK_TRANSITIONS.awaiting_review).toEqual(['completed','running','canceled'])
    expect(RESURVEY_TASK_TRANSITIONS.completed).toBeUndefined()
    expect(RESURVEY_TASK_TRANSITIONS.canceled).toBeUndefined()
  })
})
