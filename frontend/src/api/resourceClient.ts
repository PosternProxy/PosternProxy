import client from './client'

/**
 * The five CRUD functions every resource module exports. Built by
 * createResourceClient() so each module gets a typed, consistent shape
 * without 30 lines of identical boilerplate.
 */
export interface ResourceClient<T, CreateInput, UpdateInput = CreateInput> {
  /**
   * Lists items. The optional `params` object is forwarded as URL query
   * params — used by the host endpoints to filter by `?server_id=N`.
   */
  list: (params?: Record<string, string | number | boolean | undefined>) => Promise<T[]>
  get: (id: number) => Promise<T>
  create: (input: CreateInput) => Promise<T>
  update: (id: number, input: UpdateInput) => Promise<T>
  delete: (id: number) => Promise<void>
}

/**
 * createResourceClient returns the five-function CRUD shape rooted at
 * basePath. Resource-specific endpoints (enable/disable, file upload, etc.)
 * stay bespoke in the resource module — only the CRUD primitives are
 * generated.
 *
 * Example:
 *   const api = createResourceClient<ProxyHost, CreateProxyHostInput>('/proxy-hosts')
 *   export const listProxyHosts   = api.list
 *   export const getProxyHost     = api.get
 *   // …etc
 */
export function createResourceClient<T, CreateInput, UpdateInput = CreateInput>(
  basePath: string
): ResourceClient<T, CreateInput, UpdateInput> {
  return {
    async list(params) {
      const { data } = await client.get<T[]>(basePath, params ? { params } : undefined)
      return Array.isArray(data) ? data : []
    },
    async get(id) {
      const { data } = await client.get<T>(`${basePath}/${id}`)
      return data
    },
    async create(input) {
      const { data } = await client.post<T>(basePath, input)
      return data
    },
    async update(id, input) {
      const { data } = await client.put<T>(`${basePath}/${id}`, input)
      return data
    },
    async delete(id) {
      await client.delete(`${basePath}/${id}`)
    },
  }
}
