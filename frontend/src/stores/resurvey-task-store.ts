import { create } from 'zustand'
import { coverageGapApi } from '../api/coverage-gap'
import { resurveyTaskApi } from '../api/resurvey-task'
import type { CreateResurveyTask, ResurveyTask } from '../types/resurvey-task'
import type { ResurveyTaskState } from '../types/enums/resurvey-task-state'
import { useCoverageGapStore } from './coverage-gap-store'

interface ResurveyTaskStore{items:ResurveyTask[];loading:boolean;saving:boolean;fetch:(gapId:number)=>Promise<void>;reset:()=>void;create:(body:CreateResurveyTask)=>Promise<void>;transition:(task:ResurveyTask,target:ResurveyTaskState,note:string)=>Promise<void>}

async function refreshGap(gapId:number){
  const response=await coverageGapApi.get(gapId)
  useCoverageGapStore.setState(state=>({items:state.items.map(item=>item.id===gapId?response.data:item),selected:state.selected?.id===gapId?response.data:state.selected}))
}

export const useResurveyTaskStore=create<ResurveyTaskStore>((set,get)=>({items:[],loading:false,saving:false,
  fetch:async(gapId)=>{set({loading:true});try{const response=await resurveyTaskApi.list(gapId);set({items:response.data})}finally{set({loading:false})}},
  reset:()=>set({items:[]}),
  create:async(body)=>{set({saving:true});try{await resurveyTaskApi.create(body);await Promise.all([refreshGap(body.coverage_gap_id),get().fetch(body.coverage_gap_id)])}finally{set({saving:false})}},
  transition:async(task,target,note)=>{set({saving:true});try{await resurveyTaskApi.transition(task.id,target,task.version,note);await Promise.all([refreshGap(task.coverage_gap_id),get().fetch(task.coverage_gap_id)])}finally{set({saving:false})}}
}))
