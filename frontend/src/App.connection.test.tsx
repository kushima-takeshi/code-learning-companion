import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it, vi } from "vitest"
import App from "./App"

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  })
}

function pathOf(input: RequestInfo | URL): string {
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.toString()
        : input.url
  return new URL(raw).pathname
}

afterEach(() => {
  vi.unstubAllGlobals()
})

function installFetch(session: { loggedIn: boolean }) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = pathOf(input)
      if (path === "/health") {
        return jsonResponse({ status: "ok" })
      }
      if (path === "/auth/me") {
        if (!session.loggedIn) {
          return new Response(null, { status: 401 })
        }
        return jsonResponse({
          id: "user-1",
          github_login: "octocat",
          display_name: "Octo",
          avatar_url: null,
        })
      }
      if (path === "/auth/logout") {
        session.loggedIn = false
        return new Response(null, { status: 204 })
      }
      if (path === "/pr-snapshots") {
        return jsonResponse([])
      }
      if (path === "/connection-tokens" && (!init || init.method === undefined)) {
        return jsonResponse([])
      }
      throw new Error(`unexpected fetch ${path}`)
    }),
  )
}

describe("接続セクションの置き場所", () => {
  it("未ログインの単一ページには出さない", async () => {
    installFetch({ loggedIn: false })
    render(<App />)

    expect(await screen.findByRole("link", { name: "GitHubでログイン" })).toBeInTheDocument()
    expect(
      screen.queryByRole("heading", { name: "AIツールの接続" }),
    ).not.toBeInTheDocument()
  })

  it("ログイン後の取り込み履歴の下に出し、ログアウトで消す", async () => {
    const user = userEvent.setup()
    installFetch({ loggedIn: true })
    render(<App />)

    const history = await screen.findByRole("heading", { name: "取り込み履歴" })
    const connection = await screen.findByRole("heading", {
      name: "AIツールの接続",
    })
    expect(
      history.compareDocumentPosition(connection) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
    expect(screen.getByRole("heading", { name: "Code Learning Companion" })).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "ログアウト" }))
    expect(await screen.findByRole("link", { name: "GitHubでログイン" })).toBeInTheDocument()
    expect(
      screen.queryByRole("heading", { name: "AIツールの接続" }),
    ).not.toBeInTheDocument()
  })
})
