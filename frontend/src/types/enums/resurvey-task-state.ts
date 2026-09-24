export type ResurveyTaskState = 'pending' | 'in_progress' | 'pending_verification' | 'completed' | 'cancelled'
export const RESURVEY_TASK_STATE_LABEL: Record<ResurveyTaskState,string> = { pending:'待执行', in_progress:'执行中', pending_verification:'待复验', completed:'已完成', cancelled:'已取消' }
export const RESURVEY_TASK_TRANSITIONS: Partial<Record<ResurveyTaskState,ResurveyTaskState[]>> = { pending:['in_progress','cancelled'], in_progress:['pending_verification','cancelled'], pending_verification:['completed','cancelled'] }
export const RESURVEY_TASK_ACTION_LABEL: Partial<Record<ResurveyTaskState,string>> = { in_progress:'开始执行', pending_verification:'提交复验', completed:'确认完成', cancelled:'取消任务' }
