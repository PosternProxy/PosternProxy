import client from './client'

export interface ConfigurationState {
  server_id: number
  desired_revision: number
  applied_revision: number
  status: 'queued' | 'applying' | 'applied' | 'failed'
  error: string
  updated_at: string
}

export async function listConfigurationStates(): Promise<ConfigurationState[]> {
  const { data } = await client.get<ConfigurationState[]>('/configuration')
  return data
}
