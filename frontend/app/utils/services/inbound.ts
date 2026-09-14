import { z } from 'zod'
import { InboundSchema, type Inbound } from '~/utils/schemas/inbound'

interface ListInboundsResponse {
  inbounds: unknown[]
}

export async function fetchInbounds(): Promise<Inbound[]> {
  const { $api } = useNuxtApp()
  const data = await $api<ListInboundsResponse | unknown[]>('/v1/inbounds')
  const inbounds = Array.isArray(data) ? data : data.inbounds
  return z.array(InboundSchema).parse(inbounds)
}
