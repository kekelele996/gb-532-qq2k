import { create } from 'zustand'
import { resurveyTaskApi } from '../api/resurvey-task'
import { useCoverageGapStore } from './coverage-gap-store'
import type { CreateResurveyTask, ResurveyTask, ResurveyTaskView } from '../types/resurvey-task'
import type { ResurveyTaskState } from '../types/enums/resurvey-task'

interface ResurveyTaskStore {
  items:ResurveyTask[]
  loading:boolean
  busy:boolean
  fetch:(gapID?:number)=>Promise<void>
  create:(body:CreateResurveyTask)=>Promise<ResurveyTaskView>
  transition:(task:ResurveyTask,target:ResurveyTaskState,note:string)=>Promise<ResurveyTaskView>
}

function mergeTask(items:ResurveyTask[],task:ResurveyTask):ResurveyTask[] {
  return items.some(value=>value.id===task.id)
    ? items.map(value=>value.id===task.id?task:value)
    : [task,...items]
}

function applyView(view:ResurveyTaskView,items:ResurveyTask[]){
  // 同步缺口 store，保证缺口列表、人工复核面板立即反映 accepted/retesting/resurveyed。
  useCoverageGapStore.setState(state=>({
    items:state.items.map(value=>value.id===view.gap.id?view.gap:value),
    selected:state.selected?.id===view.gap.id?view.gap:state.selected
  }))
  return {items:mergeTask(items,view.task)}
}

export const useResurveyTaskStore=create<ResurveyTaskStore>((set,get)=>({
  items:[],loading:false,busy:false,
  fetch:async(gapID)=>{
    set({loading:true})
    try{
      const query=gapID?`&coverage_gap_id=${gapID}`:''
      const response=await resurveyTaskApi.list(query)
      set({items:response.data})
    }finally{set({loading:false})}
  },
  create:async(body)=>{
    if(get().busy)throw new Error('REQUEST_IN_FLIGHT')
    set({busy:true})
    try{
      const response=await resurveyTaskApi.create(body)
      set(state=>applyView(response.data,state.items))
      return response.data
    }finally{set({busy:false})}
  },
  transition:async(task,target,note)=>{
    if(get().busy)throw new Error('REQUEST_IN_FLIGHT')
    set({busy:true})
    try{
      const response=await resurveyTaskApi.transition(task.id,{target_state:target,expected_version:task.version,note})
      set(state=>applyView(response.data,state.items))
      return response.data
    }finally{set({busy:false})}
  }
}))
