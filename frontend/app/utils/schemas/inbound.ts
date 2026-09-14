import { z } from 'zod'

export const InboundSchema = z.object({
  id: z.string(),
  name: z.string(),
  type: z.string(),
  address: z.string(),
  port: z.number().int().nonnegative(),
  sni: z.string().optional().default(''),
  handshake: z.string().optional().default(''),
  public_key: z.string().optional().default(''),
  short_id: z.string().optional().default(''),
  fingerprint: z.string().optional().default(''),
  name_template: z.string().optional().default(''),
  status: z.string().optional().default(''),
  status_reason: z.string().optional().default(''),
  created_at: z.string(),
  updated_at: z.string(),
})

export type Inbound = z.infer<typeof InboundSchema>
