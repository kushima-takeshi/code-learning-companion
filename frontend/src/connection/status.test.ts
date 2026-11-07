import { describe, expect, it } from "vitest"
import { connectionStatus } from "./status"

const now = new Date("2026-11-06T00:00:00.000Z")

describe("connectionStatus", () => {
  it("期限前で失効していなければ有効", () => {
    expect(
      connectionStatus(
        {
          revoked_at: null,
          expires_at: new Date(now.getTime() + 1).toISOString(),
        },
        now,
      ),
    ).toBe("有効")
  })

  it("期限ちょうど以降は使えない", () => {
    expect(
      connectionStatus({ revoked_at: null, expires_at: now.toISOString() }, now),
    ).toBe("使えない")
  })

  it("失効済みは期限前でも使えない", () => {
    expect(
      connectionStatus(
        {
          revoked_at: "2026-11-05T00:00:00.000Z",
          expires_at: new Date(now.getTime() + 86_400_000).toISOString(),
        },
        now,
      ),
    ).toBe("使えない")
  })
})
