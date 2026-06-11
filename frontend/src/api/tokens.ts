import client from './client'

export interface APIToken {
  id: number
  name: string
  token_prefix: string // e.g. "pp_tok_a1b2c3d4" — display only
  scopes: string[]
  creator_role: string
  server_id?: number
  created_by?: number
  last_used_at?: string
  expires_at?: string
  revoked_at?: string
  created_at: string
  // Only present on the Create response:
  token?: string
}

export interface CreateTokenInput {
  name: string
  scopes: string[] // ["read"] | ["read","write"]
  expires_at?: string // ISO-8601 datetime string, or omit for never
}

export async function listTokens(): Promise<APIToken[]> {
  const { data } = await client.get<APIToken[]>('/tokens')
  return data
}

export async function createToken(input: CreateTokenInput): Promise<APIToken> {
  const { data } = await client.post<APIToken>('/tokens', input)
  return data
}

export async function revokeToken(id: number): Promise<void> {
  await client.delete(`/tokens/${id}`)
}
