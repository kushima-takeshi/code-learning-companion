import type { CurrentUser } from "./api/auth"
import { useEffect, useState } from "react"
import { apiBaseURL, fetchHealth } from "./api/health"
import { fetchMe, logout } from "./api/auth"

function App() {
  const [health, setHealth] = useState("loading")
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [authLoading, setAuthLoading] = useState(true)
  const [authError, setAuthError] = useState<string | null>(null)
  const [logoutPending, setLogoutPending] = useState(false)
  const [logoutError, setLogoutError] = useState<string | null>(null)

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
            disabled={logoutPending}
          >
            {logoutPending ? "ログアウト中…" : "ログアウト"}
          </button>

          {logoutError && <p role="alert">{logoutError}</p>}
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
