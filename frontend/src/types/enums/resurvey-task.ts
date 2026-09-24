export type ResurveyTaskState = 'pending' | 'running' | 'awaiting_review' | 'completed' | 'canceled'
export const RESURVEY_TASK_STATE_LABEL: Record<ResurveyTaskState, string> = {
  pending: '待执行', running: '执行中', awaiting_review: '待复验', completed: '已完成', canceled: '已取消'
}
export const RESURVEY_TASK_TRANSITIONS: Partial<Record<ResurveyTaskState, ResurveyTaskState[]>> = {
  pending: ['running', 'canceled'],
  running: ['awaiting_review', 'canceled'],
  awaiting_review: ['completed', 'running', 'canceled']
}
