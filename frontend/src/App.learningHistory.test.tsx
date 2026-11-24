import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, expect, it, vi } from "vitest"
import App from "./App"
import type { LearningRecordDetail, LearningRecordSummary } from "./api/learningRecords"

function installFetch(
  list: (url: URL) => Response | Promise<Response>,
  loggedIn = true,
  detail?: (url: URL) => Response | Promise<Response>,
  due: () => Response | Promise<Response> = () => Response.json({ records: [] }),
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
        return init?.method === "POST"
          ? Response.json({
              snapshot_id: "snapshot-1",
              warning: "",
              conversation_filename: null,
              title: "PRのタイトル",
              html_url: "https://github.com/owner/repo/pull/123",
              head_sha: "commit-1",
              changed_files: [],
            })
          : Response.json([])
      case "/connection-tokens":
        return Response.json([])
      case "/web/learning-records":
        expect(init?.credentials).toBe("include")
        return list(url)
      case "/web/reviews/due":
        expect(init?.credentials).toBe("include")
        expect(init?.method ?? "GET").toBe("GET")
        return due()
      default:
        if (url.pathname.startsWith("/web/learning-records/") && detail) {
          expect(init?.credentials).toBe("include")
          expect(init?.method ?? "GET").toBe("GET")
          return detail(url)
        }
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
  window.history.replaceState(null, "", "/")
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
    const links = rows.map((row) => within(row).getByRole("link", { name: /^学習テーマ/ }))
    expect(links.map((link) => link.textContent))
      .toEqual(expectedThemes[index].map((id) => `学習テーマ ${id}`))
    expect(links.map((link) => link.getAttribute("href")))
      .toEqual(expectedThemes[index].map((id) => `/learning-records/${id}`))
    for (const row of rows) {
      const time = within(row).getByText(new Date(createdAt).toLocaleString())
      expect(time).toHaveAttribute("datetime", createdAt)
    }
  })
})

it("初回取得が終わるまで読み込み中を表示する", async () => {
  installFetch(() => new Promise<Response>(() => {}))
  render(<App />)

  const history = within(await screen.findByRole("region", { name: "学習履歴" }))
  expect(history.getByRole("status")).toHaveTextContent("学習履歴を読み込んでいます")
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

const detailId = "02ffab69-b9ac-4aa7-985d-6d4eacfdedb0"
const detailPath = `/learning-records/${detailId}`
const detailRecord: LearningRecordDetail = {
  ...record(detailId, "owner/repo"),
  request_id: "c221862c-0a37-4961-be89-7f1ff5e5edd7",
  questions: [
    {
      type: "free_text",
      prompt: "defer はいつ実行されますか？",
      answer: { text: "関数を抜けるときです。", hint_used: true, code: 'defer fmt.Println("終了")' },
      feedback: { understanding: "できた", explanation: "実行のタイミングを説明できています。" },
    },
    {
      type: "free_text",
      prompt: "複数の defer の実行順は？",
      answer: { text: "登録した順です。", hint_used: false },
      feedback: { understanding: "もう一度", explanation: "最後に登録したものから実行されます。" },
    },
  ],
}

const emptyList = () => Response.json({ records: [], has_more: false })

it("期限一覧を履歴の上にAPI順で表示し、テーマから詳細を開く", async () => {
  const dueAt = "2026-11-22T10:15:00Z"
  const detail = vi.fn(() => Response.json(detailRecord))
  installFetch(emptyList, true, detail, () => Response.json({
    records: [
      { id: "another-record", theme: "SQL", stage: 1, due_at: "2026-11-21T10:15:00Z" },
      { id: detailId, theme: detailRecord.theme, repository: "owner/repo", stage: 2, due_at: dueAt },
    ],
  }))
  const user = userEvent.setup()
  render(<App />)

  const dueReviews = await screen.findByRole("region", { name: "復習期限が来ている記録" })
  const history = screen.getByRole("region", { name: "学習履歴" })
  expect(dueReviews.compareDocumentPosition(history) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  const [first, second] = await within(dueReviews).findAllByRole("listitem")
  expect(within(first).getByRole("link", { name: "SQL" })).toBeInTheDocument()
  expect(within(first).queryByText(/^リポジトリ/)).not.toBeInTheDocument()
  const link = within(second).getByRole("link", { name: detailRecord.theme })
  expect(link).toHaveAttribute("href", detailPath)
  expect(within(second).getByText("リポジトリ：owner/repo")).toBeInTheDocument()
  expect(within(second).getByText("段階 2")).toBeInTheDocument()
  expect(within(second).getByText(new Date(dueAt).toLocaleString())).toHaveAttribute("datetime", dueAt)

  await user.click(link)
  expect(window.location.pathname).toBe(detailPath)
  expect(await screen.findByRole("heading", { name: detailRecord.theme })).toBeInTheDocument()
  expect(detail).toHaveBeenCalledWith(expect.objectContaining({ pathname: `/web${detailPath}` }))
})

it("期限一覧の取得中と失敗を表示し、再試行で0件を表示する", async () => {
  const initialResponse = Promise.withResolvers<Response>()
  const due = vi.fn<() => Response | Promise<Response>>(() => Response.json({ records: [] }))
    .mockReturnValueOnce(initialResponse.promise)
  installFetch(emptyList, true, undefined, due)
  const user = userEvent.setup()
  render(<App />)

  const section = within(await screen.findByRole("region", { name: "復習期限が来ている記録" }))
  expect(section.getByRole("status")).toHaveTextContent("復習期限の一覧を読み込んでいます")
  initialResponse.resolve(Response.json({ error: "internal_error" }, { status: 500 }))
  expect(await section.findByRole("alert")).toHaveTextContent("復習期限の一覧を取得できませんでした")
  await user.click(section.getByRole("button", { name: "再試行" }))
  expect(await section.findByText("復習期限が来ている記録はありません")).toBeInTheDocument()
  expect(due).toHaveBeenCalledTimes(2)
})

it("期限一覧APIの401で既存のGitHubログインへ戻る", async () => {
  installFetch(emptyList, true, undefined, () => Response.json({ error: "unauthorized" }, { status: 401 }))
  render(<App />)

  expect(await screen.findByRole("link", { name: "GitHubでログイン" })).toBeInTheDocument()
  expect(screen.queryByRole("region", { name: "復習期限が来ている記録" })).not.toBeInTheDocument()
})

it("一覧から詳細を開き、記録の各項目と任意の回答コードを表示し、一覧へ戻れる", async () => {
  const list = vi.fn(() => Response.json({ records: [record(detailId, "owner/repo")], has_more: false }))
  const detail = vi.fn((url: URL) => {
    expect(url.pathname).toBe(`/web${detailPath}`)
    return Response.json(detailRecord)
  })
  installFetch(list, true, detail)
  const user = userEvent.setup()
  render(<App />)

  await user.click(await screen.findByRole("link", { name: detailRecord.theme }))
  expect(window.location.pathname).toBe(detailPath)
  expect(await screen.findByRole("heading", { name: detailRecord.theme })).toBeInTheDocument()
  expect(screen.getByText("リポジトリ：owner/repo")).toBeInTheDocument()
  expect(screen.getByText(new Date(createdAt).toLocaleString())).toHaveAttribute("datetime", createdAt)
  expect(screen.queryByRole("heading", { name: "学習履歴" })).not.toBeInTheDocument()

  for (const [index, question] of detailRecord.questions.entries()) {
    const panel = screen.getByRole("region", { name: `問題 ${index + 1}` })
    const content = within(panel)
    expect(content.getByText(question.prompt)).toBeInTheDocument()
    expect(content.getByText(question.answer.text)).toBeInTheDocument()
    expect(content.getByText(question.answer.hint_used ? "あり" : "なし")).toBeInTheDocument()
    expect(content.getByText(question.feedback.understanding)).toBeInTheDocument()
    expect(content.getByText(question.feedback.explanation)).toBeInTheDocument()
    if (question.answer.code) {
      expect(panel.querySelector("pre > code")?.textContent).toBe(question.answer.code)
    } else {
      expect(panel.querySelector("code")).toBeNull()
      expect(content.queryByText("回答コード")).not.toBeInTheDocument()
    }
  }

  await user.click(screen.getByRole("link", { name: "学習履歴へ戻る" }))
  expect(window.location.pathname).toBe("/")
  expect(await screen.findByRole("link", { name: detailRecord.theme })).toBeInTheDocument()
  expect(list).toHaveBeenCalledTimes(2)
  expect(detail).toHaveBeenCalledTimes(1)

  window.history.back()
  expect(await screen.findByRole("heading", { name: detailRecord.theme })).toBeInTheDocument()
  expect(window.location.pathname).toBe(detailPath)
})

it.each([
  { name: "詳細から一覧に戻った後は以前の添付を送信しない", visitDetail: true },
  { name: "取り込みフォームで選択中の添付は送信する", visitDetail: false },
])("$name", async ({ visitDetail }) => {
  const fetchMock = installFetch(
    () => Response.json({ records: [record(detailId, "owner/repo")], has_more: false }),
    true,
    () => Response.json(detailRecord),
  )
  const user = userEvent.setup()
  render(<App />)

  const prURL = "https://github.com/owner/repo/pull/123"
  const file = new File(["# テスト用の会話"], "conversation.md", { type: "text/markdown" })
  const fileLabel = "会話の Markdown ファイル（任意）"
  await user.type(await screen.findByLabelText("公開 PR の URL"), prURL)
  await user.upload(screen.getByLabelText(fileLabel), file)

  if (visitDetail) {
    await user.click(await screen.findByRole("link", { name: detailRecord.theme }))
    await user.click(await screen.findByRole("link", { name: "学習履歴へ戻る" }))
    expect(screen.getByLabelText(fileLabel)).toHaveValue("")
  }

  await user.click(screen.getByRole("button", { name: "取り込む" }))
  await screen.findByText("PR を保存しました")

  const post = fetchMock.mock.calls.find(([input, init]) =>
    new URL(input).pathname === "/pr-snapshots" && init?.method === "POST",
  )
  const body = post?.[1]?.body
  expect(body).toBeInstanceOf(FormData)
  const form = body as FormData
  expect(form.get("url")).toBe(prURL)
  expect(form.get("file")).toBe(visitDetail ? null : file)
})

it("詳細URLを直接開き、リポジトリのない記録では項目を省く", async () => {
  window.history.replaceState(null, "", detailPath)
  const list = vi.fn(emptyList)
  installFetch(list, true, () => Response.json({ ...detailRecord, repository: undefined }))
  render(<App />)

  expect(await screen.findByRole("heading", { name: detailRecord.theme })).toBeInTheDocument()
  expect(screen.queryByText(/^リポジトリ/)).not.toBeInTheDocument()
  expect(list).not.toHaveBeenCalled()
})

it("詳細取得中は読み込みを表示し、一覧へ戻れる", async () => {
  window.history.replaceState(null, "", detailPath)
  installFetch(emptyList, true, () => new Promise<Response>(() => {}))
  render(<App />)

  expect(await screen.findByRole("status")).toHaveTextContent("学習記録を読み込んでいます")
  expect(screen.getByRole("link", { name: "学習履歴へ戻る" })).toHaveAttribute("href", "/")
})

it.each([
  { status: 404, error: "record_not_found", path: detailPath, message: "学習記録が見つかりませんでした" },
  { status: 400, error: "invalid_record_id", path: "/learning-records/not-a-uuid", message: "学習記録を取得できませんでした" },
  { status: 500, error: "internal_error", path: detailPath, message: "学習記録を取得できませんでした" },
])("詳細APIの $status で $message と表示する", async ({ status, error, path, message }) => {
  window.history.replaceState(null, "", path)
  const detail = vi.fn(() => Response.json({ error }, { status }))
  installFetch(emptyList, true, detail)
  render(<App />)

  expect(await screen.findByRole("alert")).toHaveTextContent(message)
  expect(detail).toHaveBeenCalledWith(expect.objectContaining({ pathname: `/web${path}` }))
  expect(screen.getByRole("link", { name: "学習履歴へ戻る" })).toHaveAttribute("href", "/")
})

it("詳細APIの401で既存のGitHubログインへ戻る", async () => {
  window.history.replaceState(null, "", detailPath)
  const detail = vi.fn(() => Response.json({ error: "unauthorized" }, { status: 401 }))
  installFetch(emptyList, true, detail)
  render(<App />)

  expect(await screen.findByRole("link", { name: "GitHubでログイン" })).toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "学習記録の詳細" })).not.toBeInTheDocument()
  expect(detail).toHaveBeenCalledTimes(1)
})

it("未ログインで詳細URLを開いても詳細APIを呼ばない", async () => {
  window.history.replaceState(null, "", detailPath)
  const detail = vi.fn()
  installFetch(emptyList, false, detail)
  render(<App />)

  expect(await screen.findByRole("link", { name: "GitHubでログイン" })).toBeInTheDocument()
  expect(screen.queryByRole("heading", { name: "学習記録の詳細" })).not.toBeInTheDocument()
  expect(detail).not.toHaveBeenCalled()
})
