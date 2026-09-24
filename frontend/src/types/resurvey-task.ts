import type { CoverageGap } from './coverage-gap'
import type { ResurveyTaskState } from './enums/resurvey-task'

export interface ResurveyTask {
  id:number
  coverage_gap_id:number
  assignee_name:string
  planned_date:string
  task_state:ResurveyTaskState
  execution_note:string
  review_note:string
  cancel_reason:string
  created_by:number
  started_at:string|null
  submitted_at:string|null
  completed_at:string|null
  canceled_at:string|null
  version:number
  created_at:string
  updated_at:string
  coverage_gap?:CoverageGap
}

export interface ResurveyTaskView { task:ResurveyTask; gap:CoverageGap }
export interface CreateResurveyTask {
  coverage_gap_id:number
  assignee_name:string
  planned_date:string
  expected_gap_version:number
  note?:string
}
export interface ResurveyTaskTransition {
  target_state:ResurveyTaskState
  expected_version:number
  note?:string
}
