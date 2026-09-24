import { apiClient } from './client'
import type { CreateResurveyTask, ResurveyTask, ResurveyTaskTransition, ResurveyTaskView } from '../types/resurvey-task'

export const resurveyTaskApi={
  list:(query:string='')=>apiClient.page<ResurveyTask[]>(`/resurvey-tasks?page_size=100${query}`),
  get:(id:number)=>apiClient.get<ResurveyTask>(`/resurvey-tasks/${id}`),
  create:(body:CreateResurveyTask)=>apiClient.post<ResurveyTaskView>('/resurvey-tasks',body),
  transition:(id:number,body:ResurveyTaskTransition)=>apiClient.post<ResurveyTaskView>(`/resurvey-tasks/${id}/transition`,body)
}
