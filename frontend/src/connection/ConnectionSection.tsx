import { useEffect, useRef, useState, type FormEvent } from "react"
import {
  completeReconnection,
  createConnectionToken,
  getReconnection,
  listConnectionTokens,
  renameConnectionToken,
  restartReconnection,
  revokeConnectionToken,
  startReconnection,
  type ConnectionToken,
  type CreatedConnectionToken,
  type Reconnection,
  type ReconnectionResult,
} from "../api/connectionTokens"
import { connectionStatus } from "./status"
import {
  ReconnectPanel,
  type ReconnectBusy,
} from "./ReconnectPanel"
import { canRestartReconnect, visibleSecret } from "./reconnect"
import "./ConnectionSection.css"

const defaultConnectionName = "AIエージェント"

const reconnectKnownErrors: Record<string, string> = {
  token_not_found: "この接続は見つかりませんでした",
  reconnection_not_found: "この再接続は見つかりませんでした",
  reconnection_conflict: "この状態では操作できません",
  unauthorized: "ログインが必要です",
  invalid_token_id: "接続の指定が正しくありません",
  invalid_reconnection_id: "再接続の指定が正しくありません",
  invalid_reconnection_request: "再接続の要求が正しくありません",
  request_too_large: "要求が大きすぎます",
}

const tokenUnavailableMessage =
  "この接続は期限切れか失効済みのため、再接続を始められません。追加欄に同じ名前を入れました。追加するまで新しいトークンは作りません。"

export type ConnectionApi = {
  list: () => Promise<ConnectionToken[]>
  create: (name: string) => Promise<CreatedConnectionToken>
  rename: (id: string, name: string) => Promise<ConnectionToken>
  revoke: (id: string) => Promise<void>
  startReconnect: (tokenId: string) => Promise<ReconnectionResult>
  getReconnect: (id: string) => Promise<Reconnection>
  completeReconnect: (id: string) => Promise<Reconnection>
  restartReconnect: (id: string) => Promise<ReconnectionResult>
}

const defaultApi: ConnectionApi = {
  list: listConnectionTokens,
  create: createConnectionToken,
  rename: renameConnectionToken,
  revoke: revokeConnectionToken,
  startReconnect: startReconnection,
  getReconnect: getReconnection,
  completeReconnect: completeReconnection,
  restartReconnect: restartReconnection,
}

type ReconnectSession = {
  tokenId: string
  reconnection: Reconnection
  secret: string | null
}

function defaultNow(): Date {
  return new Date()
}

function defaultConfirm(message: string): boolean {
  return window.confirm(message)
}

function defaultCopy(value: string): Promise<void> {
  if (!navigator.clipboard) {
    return Promise.reject(new Error("clipboard_unavailable"))
  }
  return navigator.clipboard.writeText(value)
}

function errorText(
  err: unknown,
  fallback: string,
  known: Record<string, string>,
): string {
  const code = err instanceof Error ? err.message : ""
  return known[code] ?? fallback
}

function sessionFrom(
  tokenId: string,
  reconnection: Reconnection,
  secret: string | null,
): ReconnectSession {
  return {
    tokenId,
    reconnection,
    secret: visibleSecret(reconnection.state, secret),
  }
}

export function ConnectionSection({
  api = defaultApi,
  now = defaultNow,
  confirmRevoke = defaultConfirm,
  confirmReconnect = defaultConfirm,
  copyText = defaultCopy,
}: {
  api?: ConnectionApi
  now?: () => Date
  confirmRevoke?: (message: string) => boolean
  confirmReconnect?: (message: string) => boolean
  copyText?: (value: string) => Promise<void>
}) {
  const [name, setName] = useState(defaultConnectionName)
  const [tokens, setTokens] = useState<ConnectionToken[]>([])
  const [loading, setLoading] = useState(true)
  const [listError, setListError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [secret, setSecret] = useState<string | null>(null)
  const [secretTokenId, setSecretTokenId] = useState<string | null>(null)
  const [copyMessage, setCopyMessage] = useState<string | null>(null)
  const [copyError, setCopyError] = useState<string | null>(null)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [draftName, setDraftName] = useState("")
  const [renaming, setRenaming] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [reconnect, setReconnect] = useState<ReconnectSession | null>(null)
  const [reconnectError, setReconnectError] = useState<string | null>(null)
  const [reconnectErrorFor, setReconnectErrorFor] = useState<string | null>(null)
  const [reconnectBusy, setReconnectBusy] = useState<ReconnectBusy>(null)
  const [startingTokenId, setStartingTokenId] = useState<string | null>(null)
  const [reconnectCopyMessage, setReconnectCopyMessage] = useState<string | null>(
    null,
  )
  const [reconnectCopyError, setReconnectCopyError] = useState<string | null>(
    null,
  )
  const nameInputRef = useRef<HTMLInputElement>(null)
  const ignoreRef = useRef(false)

  async function reload() {
    try {
      const items = await api.list()
      if (ignoreRef.current) {
        return
      }
      setTokens(items)
      setListError(null)
    } catch {
      if (!ignoreRef.current) {
        setListError("接続を取得できませんでした")
      }
    }
  }

  useEffect(() => {
    let ignore = false
    ignoreRef.current = false
    api.list().then(
      (items) => {
        if (ignore) {
          return
        }
        setTokens(items)
        setListError(null)
        setLoading(false)
      },
      () => {
        if (ignore) {
          return
        }
        setListError("接続を取得できませんでした")
        setLoading(false)
      },
    )
    return () => {
      ignore = true
      ignoreRef.current = true
    }
  }, [api])

  function clearSecret() {
    setSecret(null)
    setSecretTokenId(null)
    setCopyMessage(null)
    setCopyError(null)
  }

  function closeReconnect() {
    setReconnect(null)
    setReconnectCopyMessage(null)
    setReconnectCopyError(null)
  }

  function clearReconnectError() {
    setReconnectError(null)
    setReconnectErrorFor(null)
  }

  function showReconnectError(message: string, tokenId: string | null) {
    setReconnectError(message)
    setReconnectErrorFor(tokenId)
  }

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setActionError(null)
    setCreating(true)
    try {
      const created = await api.create(name)
      if (ignoreRef.current) {
        return
      }
      setSecret(created.secret)
      setSecretTokenId(created.token.id)
      setCopyMessage(null)
      setCopyError(null)
      setName(defaultConnectionName)
      try {
        const items = await api.list()
        if (!ignoreRef.current) {
          setTokens(items)
          setListError(null)
        }
      } catch {
        if (!ignoreRef.current) {
          setListError("接続を取得できませんでした")
        }
      }
    } catch (err: unknown) {
      if (!ignoreRef.current) {
        setActionError(
          errorText(err, "接続を追加できませんでした", {
            invalid_name: "接続名を入力してください",
          }),
        )
      }
    } finally {
      if (!ignoreRef.current) {
        setCreating(false)
      }
    }
  }

  async function handleCopy() {
    if (secret === null) {
      return
    }
    try {
      await copyText(secret)
      setCopyError(null)
      setCopyMessage("コピーしました")
    } catch {
      setCopyMessage(null)
      setCopyError(
        "コピーできませんでした。表示されている文字列を選んでコピーしてください",
      )
    }
  }

  function beginRename(token: ConnectionToken) {
    setEditingId(token.id)
    setDraftName(token.name)
    setActionError(null)
  }

  async function saveRename() {
    if (editingId === null) {
      return
    }
    setActionError(null)
    setRenaming(true)
    try {
      await api.rename(editingId, draftName)
      if (ignoreRef.current) {
        return
      }
      setEditingId(null)
      await reload()
    } catch (err: unknown) {
      if (!ignoreRef.current) {
        setActionError(
          errorText(err, "名前を変えられませんでした", {
            invalid_name: "接続名を入力してください",
            token_not_found: "この接続は見つかりませんでした",
          }),
        )
      }
    } finally {
      if (!ignoreRef.current) {
        setRenaming(false)
      }
    }
  }

  async function handleRevoke(id: string) {
    const accepted = confirmRevoke(
      "この接続を失効しますか？失効すると、このトークンでは保存も取得もできません。",
    )
    if (!accepted) {
      return
    }
    setActionError(null)
    try {
      await api.revoke(id)
      if (ignoreRef.current) {
        return
      }
      if (secretTokenId === id) {
        clearSecret()
      }
      await reload()
    } catch (err: unknown) {
      if (!ignoreRef.current) {
        setActionError(
          errorText(err, "失効できませんでした", {
            token_not_found: "この接続は見つかりませんでした",
          }),
        )
      }
    }
  }

  async function beginReconnect(token: ConnectionToken) {
    clearReconnectError()
    setReconnectBusy("start")
    setStartingTokenId(token.id)
    try {
      const result = await api.startReconnect(token.id)
      if (ignoreRef.current) {
        return
      }
      const next = sessionFrom(token.id, result.reconnection, result.secret)
      setReconnect(next)
      setReconnectCopyMessage(null)
      setReconnectCopyError(null)
      if (result.reconnection.state === "completed") {
        await reload()
      }
    } catch (err: unknown) {
      if (ignoreRef.current) {
        return
      }
      const code = err instanceof Error ? err.message : ""
      if (code === "token_unavailable") {
        closeReconnect()
        setName(token.name)
        showReconnectError(tokenUnavailableMessage, null)
        nameInputRef.current?.focus()
        await reload()
        return
      }
      showReconnectError(
        errorText(err, "再接続を始められませんでした", reconnectKnownErrors),
        token.id,
      )
    } finally {
      if (!ignoreRef.current) {
        setReconnectBusy(null)
        setStartingTokenId(null)
      }
    }
  }

  function onReconnectClick(token: ConnectionToken) {
    if (reconnectBusy !== null) {
      return
    }
    if (reconnect?.tokenId === token.id) {
      clearReconnectError()
      closeReconnect()
      return
    }
    void beginReconnect(token)
  }

  async function refreshReconnect(id: string) {
    try {
      const next = await api.getReconnect(id)
      if (ignoreRef.current) {
        return
      }
      setReconnect((current) => {
        if (current === null || current.reconnection.id !== id) {
          return current
        }
        return sessionFrom(current.tokenId, next, current.secret)
      })
    } catch {
      // 読み直しに失敗しても、直前のエラー表示は残す。
    }
  }

  async function handleCheck() {
    if (reconnect === null || reconnectBusy !== null) {
      return
    }
    const id = reconnect.reconnection.id
    const tokenId = reconnect.tokenId
    setReconnectBusy("status")
    clearReconnectError()
    try {
      const next = await api.getReconnect(id)
      if (ignoreRef.current) {
        return
      }
      setReconnect((current) => {
        if (current === null || current.reconnection.id !== id) {
          return current
        }
        return sessionFrom(current.tokenId, next, current.secret)
      })
    } catch (err: unknown) {
      if (ignoreRef.current) {
        return
      }
      const code = err instanceof Error ? err.message : ""
      if (code === "reconnection_not_found") {
        closeReconnect()
        showReconnectError("この再接続は見つかりませんでした", null)
        await reload()
        return
      }
      showReconnectError(
        errorText(err, "状態を確認できませんでした", reconnectKnownErrors),
        tokenId,
      )
    } finally {
      if (!ignoreRef.current) {
        setReconnectBusy(null)
      }
    }
  }

  async function handleComplete() {
    if (
      reconnect === null ||
      reconnect.reconnection.state !== "confirmed" ||
      reconnectBusy !== null
    ) {
      return
    }
    const id = reconnect.reconnection.id
    const tokenId = reconnect.tokenId
    setReconnectBusy("complete")
    clearReconnectError()
    try {
      const next = await api.completeReconnect(id)
      if (ignoreRef.current) {
        return
      }
      setReconnect((current) => {
        if (current === null || current.reconnection.id !== id) {
          return current
        }
        return sessionFrom(current.tokenId, next, current.secret)
      })
      await reload()
    } catch (err: unknown) {
      if (ignoreRef.current) {
        return
      }
      const code = err instanceof Error ? err.message : ""
      if (code === "reconnection_conflict") {
        showReconnectError("この状態では置き換えを完了できません", tokenId)
        await refreshReconnect(id)
        return
      }
      if (code === "reconnection_not_found") {
        closeReconnect()
        showReconnectError("この再接続は見つかりませんでした", null)
        await reload()
        return
      }
      showReconnectError(
        errorText(err, "置き換えを完了できませんでした", reconnectKnownErrors),
        tokenId,
      )
    } finally {
      if (!ignoreRef.current) {
        setReconnectBusy(null)
      }
    }
  }

  async function handleRestart() {
    if (
      reconnect === null ||
      !canRestartReconnect(reconnect.reconnection.state) ||
      reconnectBusy !== null
    ) {
      return
    }
    const accepted = confirmReconnect(
      "やり直しますか？未完了の新しいトークンを失効し、新しく発行します。",
    )
    if (!accepted) {
      return
    }
    const id = reconnect.reconnection.id
    const tokenId = reconnect.tokenId
    setReconnectBusy("restart")
    clearReconnectError()
    try {
      const result = await api.restartReconnect(id)
      if (ignoreRef.current) {
        return
      }
      setReconnect(sessionFrom(tokenId, result.reconnection, result.secret))
      setReconnectCopyMessage(null)
      setReconnectCopyError(null)
    } catch (err: unknown) {
      if (ignoreRef.current) {
        return
      }
      const code = err instanceof Error ? err.message : ""
      if (code === "reconnection_conflict") {
        showReconnectError("この状態ではやり直せません", tokenId)
        await refreshReconnect(id)
        return
      }
      if (code === "reconnection_not_found") {
        closeReconnect()
        showReconnectError("この再接続は見つかりませんでした", null)
        await reload()
        return
      }
      showReconnectError(
        errorText(err, "やり直せませんでした", reconnectKnownErrors),
        tokenId,
      )
    } finally {
      if (!ignoreRef.current) {
        setReconnectBusy(null)
      }
    }
  }

  async function handleReconnectCopy() {
    const value = reconnect === null ? null : visibleSecret(
      reconnect.reconnection.state,
      reconnect.secret,
    )
    if (value === null) {
      return
    }
    try {
      await copyText(value)
      setReconnectCopyError(null)
      setReconnectCopyMessage("コピーしました")
    } catch {
      setReconnectCopyMessage(null)
      setReconnectCopyError(
        "コピーできませんでした。表示されている文字列を選んでコピーしてください",
      )
    }
  }

  function guideTarget(): ConnectionToken | null {
    if (reconnect === null) {
      return null
    }
    return tokens.find((token) => token.id === reconnect.tokenId) ?? null
  }

  const restartTarget = guideTarget()
  const errorInPanel =
    reconnect !== null &&
    reconnectError !== null &&
    reconnectErrorFor === reconnect.tokenId

  return (
    <section className="connection-section">
      <h2>AIツールの接続</h2>
      <p>
        AIツールで学習記録を保存するときに使う接続です。トークンの文字列は、追加した直後と、再接続で新しく発行した直後だけ表示します。
      </p>

      <form onSubmit={handleCreate}>
        <div>
          <label htmlFor="connection-name">接続名</label>
          <input
            id="connection-name"
            ref={nameInputRef}
            value={name}
            onChange={(event) => setName(event.target.value)}
            disabled={creating}
          />
        </div>
        <button type="submit" disabled={creating}>
          {creating ? "追加しています" : "追加する"}
        </button>
        {actionError && <p role="alert">{actionError}</p>}
      </form>

      {secret !== null && (
        <div>
          <p>このトークンは今回だけ表示します。</p>
          <p>
            <code>{secret}</code>
          </p>
          <button type="button" onClick={handleCopy}>
            コピーする
          </button>
          {copyMessage && <p role="status">{copyMessage}</p>}
          {copyError && <p role="alert">{copyError}</p>}
        </div>
      )}

      {listError && <p role="alert">{listError}</p>}
      {reconnectError && !errorInPanel && (
        <p role="alert">{reconnectError}</p>
      )}

      {loading && tokens.length === 0 ? (
        <p>接続を読み込んでいます</p>
      ) : tokens.length === 0 && !listError ? (
        <p>接続はまだありません</p>
      ) : (
        <ul className="connection-list">
          {tokens.map((token) => {
            const status = connectionStatus(token, now())
            const editing = editingId === token.id
            const open = reconnect?.tokenId === token.id
            return (
              <li key={token.id}>
                <div className="connection-row-main">
                  {editing ? (
                    <input
                      aria-label="新しい接続名"
                      value={draftName}
                      onChange={(event) => setDraftName(event.target.value)}
                    />
                  ) : (
                    <span className="connection-name">{token.name}</span>
                  )}
                  <span
                    className={
                      status === "有効"
                        ? "connection-badge"
                        : "connection-badge is-unusable"
                    }
                  >
                    {status}
                  </span>
                  <div className="connection-actions">
                    {editing && (
                      <>
                        <button
                          type="button"
                          onClick={saveRename}
                          disabled={renaming}
                        >
                          保存する
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            setEditingId(null)
                            setActionError(null)
                          }}
                        >
                          キャンセル
                        </button>
                      </>
                    )}
                    {status === "有効" ? (
                      <>
                        {!editing && (
                          <button type="button" onClick={() => beginRename(token)}>
                            名前を変える
                          </button>
                        )}
                        <button
                          type="button"
                          className={open ? "primary" : undefined}
                          aria-expanded={open}
                          aria-controls={open ? "reconnect-panel" : undefined}
                          disabled={reconnectBusy !== null}
                          onClick={() => onReconnectClick(token)}
                        >
                          {reconnectBusy === "start" && startingTokenId === token.id
                            ? "始めています"
                            : open
                              ? "再接続中"
                              : "再接続"}
                        </button>
                        <button type="button" onClick={() => handleRevoke(token.id)}>
                          失効
                        </button>
                      </>
                    ) : (
                      <button
                        type="button"
                        onClick={() => {
                          setName(token.name)
                          nameInputRef.current?.focus()
                        }}
                      >
                        作り直す
                      </button>
                    )}
                  </div>
                </div>
                {open && reconnect !== null && (
                  <ReconnectPanel
                    reconnection={reconnect.reconnection}
                    secret={reconnect.secret}
                    busy={reconnectBusy}
                    error={errorInPanel ? reconnectError : null}
                    copyMessage={reconnectCopyMessage}
                    copyError={reconnectCopyError}
                    onCheck={() => {
                      void handleCheck()
                    }}
                    onComplete={() => {
                      void handleComplete()
                    }}
                    onRestart={() => {
                      void handleRestart()
                    }}
                    onStartAgain={() => {
                      if (restartTarget !== null) {
                        void beginReconnect(restartTarget)
                      }
                    }}
                    onClose={() => {
                      clearReconnectError()
                      closeReconnect()
                    }}
                    onCopy={() => {
                      void handleReconnectCopy()
                    }}
                  />
                )}
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}
