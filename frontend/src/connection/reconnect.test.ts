import { describe, expect, it } from "vitest"
import {
  canCompleteReconnect,
  canRestartReconnect,
  formatReconnectDeadline,
  visibleSecret,
} from "./reconnect"

describe("再接続の表示判断", () => {
  it("原本を出してよいのは pending、confirmed、completed だけ", () => {
    expect(visibleSecret("pending", "clc_live")).toBe("clc_live")
    expect(visibleSecret("confirmed", "clc_live")).toBe("clc_live")
    expect(visibleSecret("completed", "clc_live")).toBe("clc_live")
    expect(visibleSecret("expired", "clc_dead")).toBeNull()
    expect(visibleSecret("superseded", "clc_dead")).toBeNull()
    expect(visibleSecret("unavailable", "clc_dead")).toBeNull()
    expect(visibleSecret("pending", null)).toBeNull()
    expect(visibleSecret("pending", "")).toBeNull()
  })

  it("完了できるのは confirmed、やり直せるのは期限内の未完了と unavailable", () => {
    expect(canCompleteReconnect("confirmed")).toBe(true)
    expect(canCompleteReconnect("pending")).toBe(false)
    expect(canCompleteReconnect("expired")).toBe(false)
    expect(canRestartReconnect("pending")).toBe(true)
    expect(canRestartReconnect("confirmed")).toBe(true)
    expect(canRestartReconnect("unavailable")).toBe(true)
    expect(canRestartReconnect("completed")).toBe(false)
    expect(canRestartReconnect("superseded")).toBe(false)
    expect(canRestartReconnect("expired")).toBe(false)
  })

  it("期限はサーバーの時刻を日本時間で表示し、解釈できない文字列はそのまま出す", () => {
    expect(formatReconnectDeadline("2026-11-08T15:00:00.000Z")).toBe(
      "2026/11/09 00:00:00 JST",
    )
    expect(formatReconnectDeadline("not-a-date")).toBe("not-a-date")
  })
})
