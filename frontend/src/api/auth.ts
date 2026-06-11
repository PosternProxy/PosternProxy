import client from './client'
import type { LoginResponse, User } from '@/types'

export async function login(email: string, password: string): Promise<LoginResponse> {
  const { data } = await client.post<LoginResponse>('/auth/login', { email, password })
  return data
}

export async function refresh(): Promise<{ access_token: string }> {
  const { data } = await client.post('/auth/refresh')
  return data
}

export async function logout(): Promise<void> {
  await client.post('/auth/logout')
}

export async function getMe(): Promise<User> {
  const { data } = await client.get<User>('/auth/me')
  return data
}
