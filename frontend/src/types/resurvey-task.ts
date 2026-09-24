import type { ResurveyTaskState } from './enums/resurvey-task-state'

export interface ResurveyTask {
  id:number; coverage_gap_id:number; assignee:string; planned_date:string; task_state:ResurveyTaskState; note:string
  version:number; created_by:number; created_at:string; updated_at:string
}
export interface CreateResurveyTask { coverage_gap_id:number; assignee:string; planned_date:string }
