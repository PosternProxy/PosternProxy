import client from './client'

export async function getSettings(): Promise<Record<string, string>> {
  const { data } = await client.get<Record<string, string>>('/settings')
  return data
}

export async function updateSettings(
  updates: Record<string, string>
): Promise<Record<string, string>> {
  const { data } = await client.put<Record<string, string>>('/settings', updates)
  return data
}
