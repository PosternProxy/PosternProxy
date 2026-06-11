import client from './client'
import type { User } from '@/types'
import { createResourceClient } from './resourceClient'

export interface CreateUserInput {
  email: string
  name: string
  password: string
  role: string
}

export type UpdateUserInput = Partial<{
  email: string
  name: string
  role: string
  is_active: boolean
}>

const api = createResourceClient<User, CreateUserInput, UpdateUserInput>('/users')

export const listUsers = api.list
export const getUser = api.get
export const createUser = api.create
export const updateUser = api.update
export const deleteUser = api.delete

// changePassword is a bespoke endpoint — not part of the CRUD shape.
export async function changePassword(id: number, password: string): Promise<void> {
  await client.put(`/users/${id}/password`, { password })
}
