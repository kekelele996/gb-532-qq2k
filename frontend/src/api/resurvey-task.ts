import { apiClient } from './client'
import type { CreateResurveyTask, ResurveyTask } from '../types/resurvey-task'
import type { ResurveyTaskState } from '../types/enums/resurvey-task-state'
export const resurveyTaskApi={
  list:(gapId:number)=>apiClient.page<ResurveyTask[]>(`/resurvey-tasks?coverage_gap_id=${gapId}&page_size=50`),
  create:(body:CreateResurveyTask)=>apiClient.post<ResurveyTask>('/resurvey-tasks',body),
  transition:(id:number,target:ResurveyTaskState,version:number,note:string)=>apiClient.post<ResurveyTask>(`/resurvey-tasks/${id}/transition`,{target_state:target,expected_version:version,note})
}
