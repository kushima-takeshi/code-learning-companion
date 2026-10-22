import type { SnapshotSummary } from "./api/prSnapshots"
import { createSnapshot, listSnapshots } from "./api/prSnapshots"
import type { CurrentUser } from "./api/auth"
import { useEffect, useState } from "react"
import { apiBaseURL, fetchHealth } from "./api/health"
import { fetchMe, logout } from "./api/auth"
import type { CreateSnapshotResponse } from "./api/prSnapshots"


function App() {
  const [health, setHealth] = useState("loading")
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [authLoading, setAuthLoading] = useState(true)
  const [authError, setAuthError] = useState<string | null>(null)
  const [logoutPending, setLogoutPending] = useState(false)
  const [logoutError, setLogoutError] = useState<string | null>(null)
  const [snapshots, setSnapshots] = useState<SnapshotSummary[]>([])
  const [snapshotsLoading, setSnapshotsLoading] = useState(false)
  const [snapshotsError, setSnapshotsError] = useState<string | null>(null)
  const [prURL, setPrURL] = useState("")
  const [conversationFile, setConversationFile] = useState<File | null>(null)
  const [importPending, setImportPending] = useState(false)
  const [importError, setImportError] = useState<string | null>(null)
  const [importResult, setImportResult] =
    useState<CreateSnapshotResponse | null>(null)

  useEffect(() => {
    let ignore = false

    fetchHealth()
      .then((body) => {
        if (!ignore) {
          setHealth(body.status)
        }
      })
      .catch((err: unknown) => {
        if (!ignore) {
          setHealth(err instanceof Error ? err.message : "request failed")
        }
      })

    return () => {
      ignore = true
    }
  }, [])

  useEffect(() => {
    let ignore = false

    async function checkAuth() {
      try {
        const currentUser = await fetchMe()

        if (!ignore) {
          setUser(currentUser)
          setAuthError(null)
        }
      } catch (err: unknown) {
        if (!ignore) {
          setAuthError(
            err instanceof Error
              ? err.message
              : "ログイン状態を確認できませんでした",
          )
        }
      } finally {
        if (!ignore) {
          setAuthLoading(false)
        }
      }
    }

    checkAuth()

    return () => {
      ignore = true
    }
  }, [])

  useEffect(() => {
    if (user === null) {
      setSnapshots([])
      setSnapshotsLoading(false)
      setSnapshotsError(null)
      return
    }

    let ignore = false

    async function loadSnapshots() {
      setSnapshotsLoading(true)
      setSnapshotsError(null)

      try {
        const items = await listSnapshots()

        if (!ignore) {
          setSnapshots(items)
        }
      } catch {
        if (!ignore) {
          setSnapshotsError("取り込み履歴を取得できませんでした")
        }
      } finally {
        if (!ignore) {
          setSnapshotsLoading(false)
        }
      }
    }

    loadSnapshots()

    return () => {
      ignore = true
    }
  }, [user])

  async function handleImport() {
    setImportPending(true)
    setImportError(null)
    setImportResult(null)

    try {
      const result = await createSnapshot(prURL, conversationFile)
      setImportResult(result)
      setSnapshotsLoading(true)
      setSnapshotsError(null)

      try {
        const items = await listSnapshots()
        setSnapshots(items)
      } catch {
        setSnapshotsError(
          "PR は保存しましたが、取り込み履歴を取得できませんでした",
        )
      } finally {
        setSnapshotsLoading(false)
      }
    } catch (err: unknown) {
      const code = err instanceof Error ? err.message : ""

      switch (code) {
        case "invalid_url":
          setImportError("GitHub の公開 PR の URL を入力してください")
          break

        case "pr_not_found":
          setImportError(
            "公開 PR のみ対応しています。URL と公開状態を確認してください",
          )
          break

        case "github_unavailable":
          setImportError(
            "GitHub から取得できませんでした。時間をおいて再試行してください",
          )
          break

        default:
          setImportError("取り込みできませんでした")
      }
    } finally {
      setImportPending(false)
    }
  }

  async function handleLogout() {
    setLogoutPending(true)
    setLogoutError(null)

    try {
      await logout()
      const currentUser = await fetchMe()
      setUser(currentUser)
    } catch (err: unknown) {
      setLogoutError(
        err instanceof Error
          ? err.message
          : "ログアウト後の状態を確認できませんでした",
      )
    } finally {
      setLogoutPending(false)
    }
  }

  return (
    <main style={{ padding: "2rem", fontFamily: "sans-serif" }}>
      <h1>Code Learning Companion</h1>
      <p>API health: {health}</p>

      {authLoading ? (
        <p>確認中</p>
      ) : authError ? (
        <p role="alert">{authError}</p>
      ) : user ? (
        <div>
          <p>{user.display_name || user.github_login}</p>

          {user.avatar_url && (
            <img
              src={user.avatar_url}
              alt="プロフィール画像"
              width={64}
              height={64}
            />
          )}

          <button
            type="button"
            onClick={handleLogout}
            disabled={logoutPending || importPending}
          >
            {logoutPending ? "ログアウト中…" : "ログアウト"}
          </button>

          {logoutError && <p role="alert">{logoutError}</p>}

          <section>
            <h2>PR を取り込む</h2>

            <form
              onSubmit={(event) => {
                event.preventDefault()
                handleImport()
              }}
            >
              <div>
                <label htmlFor="pr-url">公開 PR の URL</label>
                <input
                  id="pr-url"
                  type="url"
                  value={prURL}
                  onChange={(event) => setPrURL(event.target.value)}
                  placeholder="https://github.com/owner/repo/pull/123"
                  required
                />
              </div>

              <div>
                <label htmlFor="conversation-file">
                  会話の Markdown ファイル（任意）
                </label>
                <input
                  id="conversation-file"
                  type="file"
                  onChange={(event) =>
                    setConversationFile(event.target.files?.[0] ?? null)
                  }
                />
              </div>
              <button type="submit" disabled={importPending}>
                {importPending ? "取り込んでいます" : "取り込む"}
              </button>

              {importError && <p role="alert">{importError}</p>}

              {importResult && (
                <div>
                  <p role="status">
                    {importResult.warning
                      ? "PR は保存しました。Markdown を選び直してください"
                      : "PR を保存しました"}
                  </p>
                  <h3>{importResult.title}</h3>

                  <p>
                    <a
                      href={importResult.html_url}
                      target="_blank"
                      rel="noreferrer"
                    >
                      {importResult.html_url}
                    </a>
                  </p>

                  <p>
                    head SHA：<code>{importResult.head_sha}</code>
                  </p>
                  <h4>変更ファイル</h4>

                  {importResult.changed_files.length === 0 ? (
                    <p>変更ファイルはありません</p>
                  ) : (
                    <ul>
                      {importResult.changed_files.map((filename) => (
                        <li key={filename}>
                          <code>{filename}</code>
                        </li>
                      ))}
                    </ul>
                  )}
                  <p>
                    会話ファイル：
                    {importResult.conversation_filename ?? "保存なし"}
                  </p>
                </div>
              )}
            </form>
          </section>
          <section>
            <h2>取り込み履歴</h2>

            {snapshotsLoading ? (
              <p>履歴を読み込んでいます</p>
            ) : snapshotsError ? (
              <p role="alert">{snapshotsError}</p>
            ) : snapshots.length === 0 ? (
              <p>取り込み履歴はまだありません</p>
            ) : (
              <ul>
                {snapshots.map((snapshot) => (
                  <li key={snapshot.ID}>
                    <a
                      href={snapshot.HTMLURL}
                      target="_blank"
                      rel="noreferrer"
                    >
                      {snapshot.Title}
                    </a>
                    <p>{snapshot.HTMLURL}</p>
                    <p>
                      head SHA：<code>{snapshot.HeadSHA}</code>
                    </p>
                    <p>
                      取り込み日時：
                      {new Date(snapshot.CreatedAt).toLocaleString()}
                    </p>
                  </li>
                ))}
              </ul>
            )}
          </section>
        </div>
      ) : (
        <div>
          {new URLSearchParams(window.location.search).has("auth_error") && (
            <p role="alert">ログインできませんでした</p>
          )}

          <a href={`${apiBaseURL()}/auth/github/login`}>
            GitHubでログイン
          </a>
        </div>
      )}
    </main>
  )
}

export default App
