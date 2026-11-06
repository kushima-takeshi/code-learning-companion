import { afterEach, describe, expect, it, vi } from "vitest"
import {
  createConnectionToken,
  listConnectionTokens,
  renameConnectionToken,
  revokeConnectionToken,
} from "./connectionTokens"

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

const token = {
  id: "11111111-1111-1111-1111-111111111111",
  name: "cursor",
  created_at: "2026-11-06T00:00:00.000Z",
  expires_at: "2027-02-04T00:00:00.000Z",
  revoked_at: null,
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("connection token API", () => {
  it("一覧は本人のCookie付きでGETし、原本を要求しない", async () => {
    const fetchMock = vi.fn(async () => jsonResponse([token]))
    vi.stubGlobal("fetch", fetchMock)

    await expect(listConnectionTokens()).resolves.toEqual([token])

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8080/connection-tokens",
      { credentials: "include" },
    )
  })

  it("追加は名前だけをJSONでPOSTし、応答の原本を返す", async () => {
    const fetchMock = vi.fn(async () =>
      jsonResponse({ token, secret: "clc_secret" }, 201),
    )
    vi.stubGlobal("fetch", fetchMock)

    await expect(createConnectionToken("  cursor  ")).resolves.toEqual({
      token,
      secret: "clc_secret",
    })

    expect(fetchMock).toHaveBeenCalledWith(
      "http://localhost:8080/connection-tokens",
      {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "  cursor  " }),
      },
    )
  })

  it("名前変更は所有者の表示名だけをPOSTし、原本は返さない", async () => {
    const fetchMock = vi.fn(async () => jsonResponse({ token }))
    vi.stubGlobal("fetch", fetchMock)

    await expect(
      renameConnectionToken(token.id, "  cursor  "),
    ).resolves.toEqual(token)

    expect(fetchMock).toHaveBeenCalledWith(
      `http://localhost:8080/connection-tokens/${token.id}/rename`,
      {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "  cursor  " }),
      },
    )
  })

  it("失効は本文なしのPOSTで、204を成功にする", async () => {
    const fetchMock = vi.fn(
      async () => new Response(null, { status: 204 }),
    )
    vi.stubGlobal("fetch", fetchMock)

    await expect(revokeConnectionToken(token.id)).resolves.toBeUndefined()

    expect(fetchMock).toHaveBeenCalledWith(
      `http://localhost:8080/connection-tokens/${token.id}/revoke`,
      { method: "POST", credentials: "include" },
    )
  })

  it("失敗時はAPIのエラーコードをメッセージにする", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => jsonResponse({ error: "invalid_name" }, 400)),
    )

    await expect(createConnectionToken("")).rejects.toThrow("invalid_name")
  })

  it("JSONでない失敗はrequest_failedにする", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("nope", { status: 500 })),
    )

    await expect(listConnectionTokens()).rejects.toThrow("request_failed")
  })
})
