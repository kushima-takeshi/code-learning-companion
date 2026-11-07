import { apiBaseURL } from "./health"

export type ConnectionToken = {
  id: string
  name: string
  created_at: string
  expires_at: string
  revoked_at: string | null
}

export type CreatedConnectionToken = {
  token: ConnectionToken
  secret: string
}

async function errorCode(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as { error?: unknown }
    if (typeof body.error === "string" && body.error !== "") {
      return body.error
    }
  } catch {
    // 本文がJSONでない失敗は、下の共通コードにする。
  }
  return "request_failed"
}

export async function listConnectionTokens(): Promise<ConnectionToken[]> {
  const response = await fetch(`${apiBaseURL()}/connection-tokens`, {
    credentials: "include",
  })
  if (!response.ok) {
    throw new Error(await errorCode(response))
  }
  return (await response.json()) as ConnectionToken[]
}

export async function createConnectionToken(
  name: string,
): Promise<CreatedConnectionToken> {
  const response = await fetch(`${apiBaseURL()}/connection-tokens`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  })
  if (!response.ok) {
    throw new Error(await errorCode(response))
  }
  return (await response.json()) as CreatedConnectionToken
}

export async function renameConnectionToken(
  id: string,
  name: string,
): Promise<ConnectionToken> {
  const response = await fetch(
    `${apiBaseURL()}/connection-tokens/${encodeURIComponent(id)}/rename`,
    {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    },
  )
  if (!response.ok) {
    throw new Error(await errorCode(response))
  }
  const body = (await response.json()) as { token: ConnectionToken }
  return body.token
}

export async function revokeConnectionToken(id: string): Promise<void> {
  const response = await fetch(
    `${apiBaseURL()}/connection-tokens/${encodeURIComponent(id)}/revoke`,
    {
      method: "POST",
      credentials: "include",
    },
  )
  if (!response.ok) {
    throw new Error(await errorCode(response))
  }
}
