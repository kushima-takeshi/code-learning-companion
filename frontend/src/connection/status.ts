export function connectionStatus(
  token: { revoked_at: string | null; expires_at: string },
  now: Date,
): "有効" | "使えない" {
  if (token.revoked_at !== null) {
    return "使えない"
  }
  const expiresAt = new Date(token.expires_at).getTime()
  if (Number.isNaN(expiresAt) || expiresAt <= now.getTime()) {
    return "使えない"
  }
  return "有効"
}
