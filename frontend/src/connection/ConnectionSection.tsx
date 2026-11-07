import { useEffect, useRef, useState, type FormEvent } from "react"
import {
  createConnectionToken,
  listConnectionTokens,
  renameConnectionToken,
  revokeConnectionToken,
  type ConnectionToken,
  type CreatedConnectionToken,
} from "../api/connectionTokens"
import { connectionStatus } from "./status"

const defaultConnectionName = "AIエージェント"

export type ConnectionApi = {
  list: () => Promise<ConnectionToken[]>
  create: (name: string) => Promise<CreatedConnectionToken>
  rename: (id: string, name: string) => Promise<ConnectionToken>
  revoke: (id: string) => Promise<void>
}

const defaultApi: ConnectionApi = {
  list: listConnectionTokens,
  create: createConnectionToken,
  rename: renameConnectionToken,
  revoke: revokeConnectionToken,
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

export function ConnectionSection({
  api = defaultApi,
  now = defaultNow,
  confirmRevoke = defaultConfirm,
  copyText = defaultCopy,
}: {
  api?: ConnectionApi
  now?: () => Date
  confirmRevoke?: (message: string) => boolean
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

  return (
    <section>
      <h2>AIツールの接続</h2>
      <p>AIツールで学習記録を保存するときに使う接続です。トークンの文字列は追加したときだけ表示します。</p>

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

      {loading && tokens.length === 0 ? (
        <p>接続を読み込んでいます</p>
      ) : tokens.length === 0 && !listError ? (
        <p>接続はまだありません</p>
      ) : (
        <ul>
          {tokens.map((token) => {
            const status = connectionStatus(token, now())
            const editing = editingId === token.id
            return (
              <li key={token.id}>
                {editing ? (
                  <input
                    aria-label="新しい接続名"
                    value={draftName}
                    onChange={(event) => setDraftName(event.target.value)}
                  />
                ) : (
                  <span>{token.name}</span>
                )}
                <span>{status}</span>
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
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}
