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

export type Reconnection = {
  id: string
  old_token_id: string
  new_token_id: string
  state: string
  created_at: string
  expires_at: string
  confirmed_at: string | null
  completed_at: string | null
}

export type ReconnectionResult = {
  reconnection: Reconnection
  secret: string | null
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

function readReconnectionResult(body: unknown): ReconnectionResult {
  if (typeof body !== "object" || body === null || !("reconnection" in body)) {
    throw new Error("request_failed")
  }
  const record = body as { reconnection?: unknown; secret?: unknown }
  if (typeof record.reconnection !== "object" || record.reconnection === null) {
    throw new Error("request_failed")
  }
  const secret =
    typeof record.secret === "string" && record.secret !== ""
      ? record.secret
      : null
  return { reconnection: record.reconnection as Reconnection, secret }
}

async function postReconnection(path: string): Promise<ReconnectionResult> {
  const response = await fetch(`${apiBaseURL()}${path}`, {
    method: "POST",
    credentials: "include",
  })
  if (!response.ok) {
    throw new Error(await errorCode(response))
  }
  return readReconnectionResult(await response.json())
}

export async function startReconnection(
  tokenId: string,
): Promise<ReconnectionResult> {
  return postReconnection(
    `/connection-tokens/${encodeURIComponent(tokenId)}/reconnections`,
  )
}

export async function getReconnection(id: string): Promise<Reconnection> {
  const response = await fetch(
    `${apiBaseURL()}/connection-reconnections/${encodeURIComponent(id)}`,
    { credentials: "include" },
  )
  if (!response.ok) {
    throw new Error(await errorCode(response))
  }
  // 状態取得の応答に secret が混ざっても、原本としては返さない。
  return readReconnectionResult(await response.json()).reconnection
}

export async function completeReconnection(id: string): Promise<Reconnection> {
  const result = await postReconnection(
    `/connection-reconnections/${encodeURIComponent(id)}/complete`,
  )
  return result.reconnection
}

export async function restartReconnection(
  id: string,
): Promise<ReconnectionResult> {
  return postReconnection(
    `/connection-reconnections/${encodeURIComponent(id)}/restart`,
  )
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
