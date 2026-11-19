import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, it, vi } from "vitest"
import App from "./App"
import type { LearningRecordSummary } from "./api/learningRecords"

function installFetch(
  list: (url: URL) => Response | Promise<Response>,
  loggedIn = true,
) {
  const fetchMock = vi.fn(async (input: string, init?: RequestInit) => {
    const url = new URL(input)
    switch (url.pathname) {
      case "/health":
        return Response.json({ status: "ok" })
      case "/auth/me":
        return loggedIn
          ? Response.json({
              id: "user-1",
              github_login: "octocat",
              display_name: "Octo",
              avatar_url: null,
            })
          : new Response(null, { status: 401 })
      case "/pr-snapshots":
      case "/connection-tokens":
        return Response.json([])
      case "/web/learning-records":
        expect(init?.credentials).toBe("include")
        return list(url)
      default:
        throw new Error(`unexpected fetch ${url.pathname}`)
    }
  })
  vi.stubGlobal("fetch", fetchMock)
  return fetchMock
}

const createdAt = "2026-11-19T10:15:00Z"

function record(id: string, repository?: string): LearningRecordSummary {
  return { id, theme: `学習テーマ ${id}`, created_at: createdAt, repository }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

it("リポジトリ別にまとめ、テーマと保存日時を表示する", async () => {
  installFetch(() => Response.json({
    records: [
      record("5"),
      record("4", "owner/z"),
      record("3", "owner/a"),
      record("2", "owner/z"),
      record("1", ""),
    ],
    has_more: false,
  }))
  render(<App />)

  const history = await screen.findByRole("region", { name: "学習履歴" })
  const importHeading = screen.getByRole("heading", { name: "PR を取り込む" })
  expect(history.compareDocumentPosition(importHeading) & Node.DOCUMENT_POSITION_FOLLOWING)
    .toBeTruthy()
  await within(history).findByRole("heading", { name: "owner/z" })
  const headings = within(history).getAllByRole("heading", { level: 3 })
  expect(headings.map((heading) => heading.textContent)).toEqual([
    "owner/z", "owner/a", "リポジトリなし",
  ])
  const expectedThemes = [["4", "2"], ["3"], ["5", "1"]]
  headings.forEach((heading, index) => {
    const rows = within(heading.parentElement!).getAllByRole("listitem")
    expect(rows.map((row) => within(row).getByText(/^学習テーマ/).textContent))
      .toEqual(expectedThemes[index].map((id) => `学習テーマ ${id}`))
    for (const row of rows) {
      const time = within(row).getByText(new Date(createdAt).toLocaleString())
      expect(time).toHaveAttribute("datetime", createdAt)
    }
  })
})

it("初回取得が終わるまで読み込み中を表示する", async () => {
  installFetch(() => new Promise<Response>(() => {}))
  render(<App />)

  expect(await screen.findByRole("status")).toHaveTextContent("学習履歴を読み込んでいます")
  expect(screen.queryByText("学習記録はまだありません")).not.toBeInTheDocument()
})

it("0件のときは空の状態を表示する", async () => {
  installFetch(() => Response.json({ records: [], has_more: false }))
  render(<App />)

  expect(await screen.findByText("学習記録はまだありません")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "さらに表示" })).not.toBeInTheDocument()
})

it("取得失敗を表示し、同じページを再試行できる", async () => {
  const list = vi.fn(() => Response.json({ records: [], has_more: false }))
    .mockImplementationOnce(() => Response.json({ error: "internal_error" }, { status: 500 }))
  const user = userEvent.setup()
  installFetch(list)
  render(<App />)

  expect(await screen.findByRole("alert")).toHaveTextContent("学習履歴を取得できませんでした")
  await user.click(screen.getByRole("button", { name: "再試行" }))
  expect(await screen.findByText("学習記録はまだありません")).toBeInTheDocument()
  expect(list.mock.calls).toHaveLength(2)
})

it("履歴APIの401で既存のGitHubログインへ戻る", async () => {
  installFetch(() => Response.json({ error: "unauthorized" }, { status: 401 }))
  render(<App />)

  expect(await screen.findByRole("link", { name: "GitHubでログイン" })).toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "学習履歴" })).not.toBeInTheDocument()
})

it("続きがあれば次のページを取得して既存のグループへ追加する", async () => {
  const offsets: string[] = []
  installFetch((url) => {
    offsets.push(url.search)
    if (!url.search) {
      return Response.json({
        records: Array.from({ length: 20 }, (_, index) => record(String(20 - index), "owner/repo")),
        has_more: true,
      })
    }
    return Response.json({
      records: [record("1", "owner/repo"), record("0", "owner/repo")],
      has_more: false,
    })
  })
  const user = userEvent.setup()
  render(<App />)

  await user.click(await screen.findByRole("button", { name: "さらに表示" }))
  expect(await screen.findByText("学習テーマ 0")).toBeInTheDocument()
  const history = screen.getByRole("region", { name: "学習履歴" })
  expect(within(history).getAllByRole("heading", { name: "owner/repo" })).toHaveLength(1)
  expect(within(history).getAllByRole("listitem")).toHaveLength(21)
  expect(within(history).getByText("学習テーマ 20")).toBeInTheDocument()
  expect(screen.queryByRole("button", { name: "さらに表示" })).not.toBeInTheDocument()
  expect(offsets).toEqual(["", "?limit=20&offset=20"])
})

it("未ログインでは履歴を表示せず、一覧APIも呼ばない", async () => {
  const list = vi.fn()
  installFetch(list, false)
  render(<App />)

  expect(await screen.findByRole("link", { name: "GitHubでログイン" })).toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "学習履歴" })).not.toBeInTheDocument()
  expect(list).not.toHaveBeenCalled()
})
