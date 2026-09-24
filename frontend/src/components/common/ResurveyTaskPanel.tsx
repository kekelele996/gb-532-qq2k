import AssignmentTurnedInOutlined from '@mui/icons-material/AssignmentTurnedInOutlined'
import PlayArrowRounded from '@mui/icons-material/PlayArrowRounded'
import FactCheckOutlined from '@mui/icons-material/FactCheckOutlined'
import UndoRounded from '@mui/icons-material/UndoRounded'
import CancelOutlined from '@mui/icons-material/CancelOutlined'
import { Alert, Box, Button, Chip, IconButton, Stack, TextField, Tooltip, Typography } from '@mui/material'
import { useMemo, useState } from 'react'
import { useAuth } from '../../hooks/useAuth'
import { useResurveyTaskStore } from '../../stores/resurvey-task-store'
import type { CoverageGap } from '../../types/coverage-gap'
import type { ResurveyTask } from '../../types/resurvey-task'
import { RESURVEY_TASK_STATE_LABEL, RESURVEY_TASK_TRANSITIONS, type ResurveyTaskState } from '../../types/enums/resurvey-task'

interface Props { gap:CoverageGap; onChanged:()=>void }

function todayISO(){return new Date().toISOString().slice(0,10)}

export function ResurveyTaskPanel({gap,onChanged}:Props){
  const {hasRole}=useAuth()
  const {items,busy,fetch,create,transition}=useResurveyTaskStore()
  const [assignee,setAssignee]=useState('')
  const [plannedDate,setPlannedDate]=useState(todayISO())
  const [createNote,setCreateNote]=useState('')
  const [formError,setFormError]=useState('')
  const [actionNote,setActionNote]=useState<Record<number,string>>({})

  const gapTasks=useMemo(()=>items.filter(task=>task.coverage_gap_id===gap.id),[items,gap.id])
  const openTask=gapTasks.find(task=>RESURVEY_TASK_TRANSITIONS[task.task_state])
  const othersOpen=useMemo(()=>items.filter(task=>task.coverage_gap_id!==gap.id&&RESURVEY_TASK_TRANSITIONS[task.task_state]).slice(0,6),[items,gap.id])

  const canCreate=hasRole('admin','reviewer')
  const canExecute=hasRole('admin','data_processor')
  const canVerify=hasRole('admin','reviewer')

  const submitCreate=async()=>{
    setFormError('')
    if(assignee.trim().length<2){setFormError('请填写执行人（至少 2 个字符）');return}
    if(!plannedDate){setFormError('请选择计划日期');return}
    try{
      await create({coverage_gap_id:gap.id,assignee_name:assignee.trim(),planned_date:plannedDate,expected_gap_version:gap.version,note:createNote.trim()||undefined})
      setAssignee('');setCreateNote('');onChanged()
    }catch{ /* 全局错误条已展示冲突提示 */ }
  }

  const advance=async(task:ResurveyTask,target:ResurveyTaskState)=>{
    const note=(actionNote[task.id]??'').trim()
    if((target==='completed'||target==='canceled')&&note.length<4)return
    try{
      await transition(task,target,note)
      setActionNote(state=>({...state,[task.id]:''}))
      onChanged()
    }catch{ /* 旧版本/重复点击冲突由全局错误条提示 */ }
  }

  const roleMay=(target:ResurveyTaskState)=>
    (target==='running'||target==='awaiting_review'?canExecute:canVerify)

  return <Box className="resurvey-panel">
    <Typography variant="overline">RESURVEY EXECUTION</Typography>
    <Typography variant="h6">补测执行单</Typography>
    <Typography variant="caption" color="text.secondary">仅“已接受补测”的缺口可建单；同一缺口同时只允许一张未结束任务。</Typography>

    {gap.gap_state==='accepted'&&(
      <Stack spacing={1.25} sx={{mt:1.5}} component="form" onSubmit={event=>{event.preventDefault();void submitCreate()}}>
        {formError&&<Alert severity="error">{formError}</Alert>}
        <TextField size="small" fullWidth label="执行人" value={assignee} onChange={event=>setAssignee(event.target.value)} inputProps={{maxLength:80}} placeholder="姓名或工号"/>
        <TextField size="small" fullWidth type="date" label="计划日期" InputLabelProps={{shrink:true}} value={plannedDate} inputProps={{min:todayISO()}} onChange={event=>setPlannedDate(event.target.value)}/>
        <TextField size="small" fullWidth multiline minRows={2} label="补测说明（可选）" value={createNote} onChange={event=>setCreateNote(event.target.value)} inputProps={{maxLength:500}}/>
        <Button type="submit" variant="contained" disabled={!canCreate||busy} startIcon={<AssignmentTurnedInOutlined/>}>建立补测单</Button>
      </Stack>
    )}

    {gapTasks.length===0
      ? <Typography variant="body2" color="text.secondary" sx={{mt:1.5}}>该缺口暂无补测执行单。</Typography>
      : <Stack spacing={1.25} sx={{mt:1.5}}>
        {gapTasks.map(task=>{
          const targets=RESURVEY_TASK_TRANSITIONS[task.task_state]??[]
          const note=(actionNote[task.id]??'')
          const noteRequired=targets.some(target=>(target==='completed'||target==='canceled'))
          return <Box key={task.id} className="task-card">
            <Box className="task-card-head">
              <strong>补测单 #{task.id}</strong>
              <Chip size="small" color={task.task_state==='completed'?'success':task.task_state==='canceled'?'default':'warning'} variant={task.task_state==='canceled'?'outlined':'filled'} label={RESURVEY_TASK_STATE_LABEL[task.task_state]}/>
            </Box>
            <dl>
              <dt>执行人</dt><dd>{task.assignee_name}</dd>
              <dt>计划日期</dt><dd>{task.planned_date.slice(0,10)}</dd>
              {task.started_at&&<><dt>开始</dt><dd>{new Date(task.started_at).toLocaleString('zh-CN')}</dd></>}
              {task.submitted_at&&<><dt>提交复验</dt><dd>{new Date(task.submitted_at).toLocaleString('zh-CN')}</dd></>}
              {task.execution_note&&<><dt>执行记录</dt><dd>{task.execution_note}</dd></>}
              {task.review_note&&<><dt>复验结论</dt><dd>{task.review_note}</dd></>}
              {task.cancel_reason&&<><dt>取消原因</dt><dd>{task.cancel_reason}</dd></>}
            </dl>
            {targets.length>0&&<>
              <TextField size="small" fullWidth multiline minRows={2} label={noteRequired?'复验结论 / 取消原因（至少 4 个字符）':'执行备注（可选）'} value={note} onChange={event=>setActionNote(state=>({...state,[task.id]:event.target.value}))} inputProps={{maxLength:500}} sx={{mt:0.5}}/>
              <Stack direction="row" spacing={1} sx={{mt:0.5,flexWrap:'wrap',gap:1}}>
                {targets.map(target=>{
                  const Icon=target==='running'?(openTask?.task_state==='awaiting_review'?UndoRounded:PlayArrowRounded):target==='awaiting_review'?AssignmentTurnedInOutlined:target==='completed'?FactCheckOutlined:CancelOutlined
                  const blocked=(target==='completed'||target==='canceled')&&note.trim().length<4
                  return <Tooltip key={target} title={busy?'操作进行中，请勿重复点击':''}>
                    <span><Button size="small" variant={target==='canceled'?'outlined':'contained'} color={target==='canceled'?'error':target==='completed'?'success':'primary'} startIcon={<Icon/>} disabled={busy||blocked||!roleMay(target)} onClick={()=>void advance(task,target)}>{RESURVEY_TASK_STATE_LABEL[target]}</Button></span>
                  </Tooltip>
                })}
              </Stack>
            </>}
          </Box>
        })}
      </Stack>}

    {othersOpen.length>0&&<Box sx={{mt:2}}>
      <Typography variant="overline" color="text.secondary">其他缺口未结束任务</Typography>
      <Stack spacing={0.5} sx={{mt:0.5}}>
        {othersOpen.map(task=><Box key={task.id} className="task-mini">
          <IconButton size="small" onClick={()=>void fetch()} aria-label="刷新任务列表"><UndoRounded fontSize="inherit"/></IconButton>
          <span>#{task.coverage_gap_id} 缺口 · {task.assignee_name} · {task.planned_date.slice(0,10)}</span>
          <Chip size="small" variant="outlined" label={RESURVEY_TASK_STATE_LABEL[task.task_state]}/>
        </Box>)}
      </Stack>
    </Box>}
  </Box>
}
