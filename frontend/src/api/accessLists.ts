import type { AccessList } from '@/types'
import { createResourceClient } from './resourceClient'

export interface AccessListClientInput {
  address: string
  directive: 'allow' | 'deny'
  sort_order: number
}

export interface CreateAccessListInput {
  name: string
  clients: AccessListClientInput[]
}

const api = createResourceClient<AccessList, CreateAccessListInput>('/access-lists')

export const listAccessLists = api.list
export const getAccessList = api.get
export const createAccessList = api.create
export const updateAccessList = api.update
export const deleteAccessList = api.delete
