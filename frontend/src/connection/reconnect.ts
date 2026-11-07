import type { Reconnection } from "../api/connectionTokens"

const visibleStates = new Set(["pending", "confirmed", "completed"])

export function visibleSecret(
  state: string,
  secret: string | null,
): string | null {
  if (secret === null || secret === "") {
    return null
  }
  if (!visibleStates.has(state)) {
    return null
  }
  return secret
}

export function canCompleteReconnect(state: string): boolean {
  return state === "confirmed"
}

export function canRestartReconnect(state: string): boolean {
  return state === "pending" || state === "confirmed" || state === "unavailable"
}

export function reconnectStateLabel(state: string): string {
  switch (state) {
    case "pending":
      return "接続確認待ち（pending）"
    case "confirmed":
      return "接続確認済み（confirmed）"
    case "completed":
      return "置き換え完了（completed）"
    case "superseded":
      return "やり直し済み（superseded）"
    case "expired":
      return "期限切れ（expired）"
    case "unavailable":
      return "新しいトークンは使えません（unavailable）"
    default:
      return `未対応の状態（${state}）`
  }
}

export function formatReconnectDeadline(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) {
    return iso
  }
  const parts = new Intl.DateTimeFormat("ja-JP", {
    timeZone: "Asia/Tokyo",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  }).formatToParts(date)
  const pick = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((part) => part.type === type)?.value ?? ""
  return `${pick("year")}/${pick("month")}/${pick("day")} ${pick("hour")}:${pick("minute")}:${pick("second")} JST`
}

export function isActiveReconnect(reconnection: Reconnection): boolean {
  return reconnection.state === "pending" || reconnection.state === "confirmed"
}
