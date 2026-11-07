import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import {
  ConnectionSection,
  type ConnectionApi,
} from "./ConnectionSection"
import type { ConnectionToken } from "../api/connectionTokens"

const now = new Date("2026-11-06T00:00:00.000Z")

function token(overrides: Partial<ConnectionToken> = {}): ConnectionToken {
  return {
    id: "11111111-1111-1111-1111-111111111111",
    name: "codex",
    created_at: "2026-10-01T00:00:00.000Z",
    expires_at: "2027-02-04T00:00:00.000Z",
    revoked_at: null,
    ...overrides,
  }
}

function deferred<T>() {
  let resolve: (value: T) => void = () => {}
  let reject: (reason?: unknown) => void = () => {}
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe("AIツールの接続", () => {
  let api: {
    list: ReturnType<typeof vi.fn<ConnectionApi["list"]>>
    create: ReturnType<typeof vi.fn<ConnectionApi["create"]>>
    rename: ReturnType<typeof vi.fn<ConnectionApi["rename"]>>
    revoke: ReturnType<typeof vi.fn<ConnectionApi["revoke"]>>
  }
  let confirmRevoke: ReturnType<typeof vi.fn<(message: string) => boolean>>
  let copyText: ReturnType<typeof vi.fn<(value: string) => Promise<void>>>

  beforeEach(() => {
    localStorage.clear()
    api = {
      list: vi.fn(async () => []),
      create: vi.fn(),
      rename: vi.fn(),
      revoke: vi.fn(async () => {}),
    }
    confirmRevoke = vi.fn(() => true)
    copyText = vi.fn(async () => {})
  })

  function renderSection() {
    return render(
      <ConnectionSection
        api={api}
        now={() => now}
        confirmRevoke={confirmRevoke}
        copyText={copyText}
      />,
    )
  }

  it("初期値はAIエージェントで、接続が無ければその旨を出す", async () => {
    renderSection()

    expect(
      screen.getByRole("heading", { name: "AIツールの接続" }),
    ).toBeInTheDocument()
    expect(screen.getByLabelText("接続名")).toHaveValue("AIエージェント")
    expect(await screen.findByText("接続はまだありません")).toBeInTheDocument()
    expect(localStorage.length).toBe(0)
  })

  it("使える行は有効で名前変更と失効、使えない行は作り直すだけ", async () => {
    const usable = token()
    const expired = token({
      id: "22222222-2222-2222-2222-222222222222",
      name: "期限切れ",
      expires_at: now.toISOString(),
    })
    const revoked = token({
      id: "33333333-3333-3333-3333-333333333333",
      name: "失効済み",
      revoked_at: "2026-11-01T00:00:00.000Z",
    })
    api.list.mockResolvedValue([usable, expired, revoked])
    renderSection()

    const usableRow = within((await screen.findByText("codex")).closest("li")!)
    const expiredRow = within(screen.getByText("期限切れ").closest("li")!)
    const revokedRow = within(screen.getByText("失効済み").closest("li")!)
    expect(usableRow.getByText("有効")).toBeInTheDocument()
    expect(usableRow.getByRole("button", { name: "名前を変える" })).toBeInTheDocument()
    expect(usableRow.getByRole("button", { name: "再接続" })).toBeInTheDocument()
    expect(usableRow.getByRole("button", { name: "失効" })).toBeInTheDocument()
    expect(usableRow.queryByRole("button", { name: "作り直す" })).not.toBeInTheDocument()
    expect(expiredRow.getByText("使えない")).toBeInTheDocument()
    expect(expiredRow.getByRole("button", { name: "作り直す" })).toBeInTheDocument()
    expect(expiredRow.queryByRole("button", { name: "再接続" })).not.toBeInTheDocument()
    expect(expiredRow.queryByRole("button", { name: "名前を変える" })).not.toBeInTheDocument()
    expect(expiredRow.queryByRole("button", { name: "失効" })).not.toBeInTheDocument()
    expect(revokedRow.getByText("使えない")).toBeInTheDocument()
    expect(revokedRow.getByRole("button", { name: "作り直す" })).toBeInTheDocument()
    expect(revokedRow.queryByRole("button", { name: "名前を変える" })).not.toBeInTheDocument()
    expect(revokedRow.queryByRole("button", { name: "失効" })).not.toBeInTheDocument()
  })

  it("追加すると原本を一度だけ出し、一覧には含めず、次の追加で置き換える", async () => {
    const user = userEvent.setup()
    const first = token({ name: "codex" })
    const second = token({
      id: "22222222-2222-2222-2222-222222222222",
      name: "cursor",
    })
    api.list
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([first])
      .mockResolvedValueOnce([first, second])
    api.create
      .mockResolvedValueOnce({ token: first, secret: "clc_first" })
      .mockResolvedValueOnce({ token: second, secret: "clc_second" })
    renderSection()
    await screen.findByText("接続はまだありません")

    await user.clear(screen.getByLabelText("接続名"))
    await user.type(screen.getByLabelText("接続名"), "  codex  ")
    await user.click(screen.getByRole("button", { name: "追加する" }))

    expect(api.create).toHaveBeenCalledWith("  codex  ")
    expect(await screen.findByText("clc_first")).toBeInTheDocument()
    expect(screen.getAllByText("clc_first")).toHaveLength(1)
    expect(screen.getByRole("button", { name: "コピーする" })).toBeInTheDocument()
    expect(screen.queryByText(/mcp|config/i)).not.toBeInTheDocument()
    expect(screen.getByLabelText("接続名")).toHaveValue("AIエージェント")
    expect(await screen.findByText("codex")).toBeInTheDocument()
    expect(localStorage.length).toBe(0)

    await user.clear(screen.getByLabelText("接続名"))
    await user.type(screen.getByLabelText("接続名"), "cursor")
    await user.click(screen.getByRole("button", { name: "追加する" }))

    expect(await screen.findByText("clc_second")).toBeInTheDocument()
    expect(screen.queryByText("clc_first")).not.toBeInTheDocument()
    expect(screen.getAllByText("clc_second")).toHaveLength(1)
  })

  it("一覧の再取得に失敗しても、発行した原本は残す", async () => {
    const user = userEvent.setup()
    api.list.mockResolvedValueOnce([]).mockRejectedValueOnce(new Error("list_failed"))
    api.create.mockResolvedValue({ token: token(), secret: "clc_kept" })
    renderSection()
    await screen.findByText("接続はまだありません")

    await user.click(screen.getByRole("button", { name: "追加する" }))

    expect(await screen.findByText("clc_kept")).toBeInTheDocument()
    expect(
      await screen.findByRole("alert"),
    ).toHaveTextContent("接続を取得できませんでした")
  })

  it("コピーできたときと、できなかったときを知らせる", async () => {
    const user = userEvent.setup()
    api.create.mockResolvedValue({ token: token(), secret: "clc_copy" })
    renderSection()
    await screen.findByText("接続はまだありません")
    await user.click(screen.getByRole("button", { name: "追加する" }))
    await screen.findByText("clc_copy")

    await user.click(screen.getByRole("button", { name: "コピーする" }))
    expect(copyText).toHaveBeenCalledWith("clc_copy")
    expect(await screen.findByRole("status")).toHaveTextContent("コピーしました")

    copyText.mockRejectedValueOnce(new Error("denied"))
    await user.click(screen.getByRole("button", { name: "コピーする" }))
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "コピーできませんでした。表示されている文字列を選んでコピーしてください",
    )
    expect(screen.getByText("clc_copy")).toBeInTheDocument()
  })

  it("作り直すは同じ名前を追加欄へ入れるだけで、発行しない", async () => {
    const user = userEvent.setup()
    api.list.mockResolvedValue([
      token({ name: "codex", expires_at: now.toISOString() }),
    ])
    renderSection()
    await user.click(await screen.findByRole("button", { name: "作り直す" }))

    expect(screen.getByLabelText("接続名")).toHaveValue("codex")
    expect(screen.getByLabelText("接続名")).toHaveFocus()
    expect(api.create).not.toHaveBeenCalled()
  })

  it("名前を変えるは保存するまでAPIを呼ばず、別の行へ移ると下書きを捨てる", async () => {
    const user = userEvent.setup()
    const first = token({ name: "codex" })
    const second = token({
      id: "22222222-2222-2222-2222-222222222222",
      name: "cursor",
    })
    api.list.mockImplementation(async () => {
      if (api.rename.mock.calls.length > 0) {
        return [first, { ...second, name: "renamed" }]
      }
      return [first, second]
    })
    api.rename.mockResolvedValue({ ...second, name: "renamed" })
    renderSection()

    const renameButtons = await screen.findAllByRole("button", {
      name: "名前を変える",
    })
    await user.click(renameButtons[0])
    const draft = screen.getByLabelText("新しい接続名")
    expect(draft).toHaveValue("codex")
    await user.clear(draft)
    await user.type(draft, "下書き")
    await user.click(screen.getByRole("button", { name: "キャンセル" }))
    expect(api.rename).not.toHaveBeenCalled()
    expect(screen.getByText("codex")).toBeInTheDocument()

    await user.click(screen.getAllByRole("button", { name: "名前を変える" })[0])
    await user.clear(screen.getByLabelText("新しい接続名"))
    await user.type(screen.getByLabelText("新しい接続名"), "下書き")
    await user.click(screen.getByRole("button", { name: "名前を変える" }))
    expect(screen.getByLabelText("新しい接続名")).toHaveValue("cursor")
    expect(screen.getByText("codex")).toBeInTheDocument()
    expect(api.rename).not.toHaveBeenCalled()

    await user.clear(screen.getByLabelText("新しい接続名"))
    await user.type(screen.getByLabelText("新しい接続名"), "  renamed  ")
    await user.click(screen.getByRole("button", { name: "保存する" }))
    expect(api.rename).toHaveBeenCalledWith(second.id, "  renamed  ")
    expect(await screen.findByText("renamed")).toBeInTheDocument()
  })

  it("空の名前は追加も名前変更も拒否する", async () => {
    const user = userEvent.setup()
    api.list.mockResolvedValue([token()])
    api.create.mockRejectedValue(new Error("invalid_name"))
    api.rename.mockRejectedValue(new Error("invalid_name"))
    renderSection()
    await screen.findByText("codex")

    await user.clear(screen.getByLabelText("接続名"))
    await user.click(screen.getByRole("button", { name: "追加する" }))
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "接続名を入力してください",
    )
    expect(screen.queryByRole("button", { name: "コピーする" })).not.toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "名前を変える" }))
    await user.clear(screen.getByLabelText("新しい接続名"))
    await user.click(screen.getByRole("button", { name: "保存する" }))
    await waitFor(() => expect(api.rename).toHaveBeenCalled())
    expect(screen.getByRole("alert")).toHaveTextContent(
      "接続名を入力してください",
    )
    expect(screen.getByLabelText("新しい接続名")).toBeInTheDocument()
  })

  it("失効は確認を取り、取り消したら要求せず、表示中の原本は失効後に消す", async () => {
    const user = userEvent.setup()
    const created = token()
    api.list.mockImplementation(async () => {
      if (api.revoke.mock.calls.length > 0) {
        return [token({ revoked_at: "2026-11-06T00:00:00.000Z" })]
      }
      if (api.create.mock.calls.length > 0) {
        return [created]
      }
      return []
    })
    api.create.mockResolvedValue({ token: created, secret: "clc_live" })
    renderSection()
    await screen.findByText("接続はまだありません")
    await user.click(screen.getByRole("button", { name: "追加する" }))
    await screen.findByText("clc_live")
    await screen.findByRole("button", { name: "失効" })

    confirmRevoke.mockReturnValueOnce(false)
    await user.click(screen.getByRole("button", { name: "失効" }))
    expect(confirmRevoke).toHaveBeenCalled()
    expect(String(confirmRevoke.mock.calls[0][0])).toContain("保存")
    expect(String(confirmRevoke.mock.calls[0][0])).toContain("取得")
    expect(api.revoke).not.toHaveBeenCalled()
    expect(screen.getByText("clc_live")).toBeInTheDocument()

    confirmRevoke.mockReturnValueOnce(true)
    await user.click(screen.getByRole("button", { name: "失効" }))
    expect(api.revoke).toHaveBeenCalledWith(created.id)
    expect(await screen.findByText("使えない")).toBeInTheDocument()
    expect(screen.queryByText("clc_live")).not.toBeInTheDocument()
  })

  it("追加の途中ではボタンを押せない", async () => {
    const user = userEvent.setup()
    const pending = deferred<{ token: ConnectionToken; secret: string }>()
    api.create.mockReturnValue(pending.promise)
    renderSection()
    await screen.findByText("接続はまだありません")

    await user.click(screen.getByRole("button", { name: "追加する" }))
    expect(screen.getByRole("button", { name: "追加しています" })).toBeDisabled()
    expect(api.create).toHaveBeenCalledTimes(1)

    pending.resolve({ token: token(), secret: "clc_done" })
    expect(await screen.findByText("clc_done")).toBeInTheDocument()
  })

  it("表示を閉じると原本は残らない", async () => {
    const user = userEvent.setup()
    function Host() {
      const [open, setOpen] = useState(true)
      return open ? (
        <>
          <button type="button" onClick={() => setOpen(false)}>
            閉じる
          </button>
          <ConnectionSection
            api={api}
            now={() => now}
            confirmRevoke={confirmRevoke}
            copyText={copyText}
          />
        </>
      ) : (
        <p>閉じた</p>
      )
    }
    api.create.mockResolvedValue({ token: token(), secret: "clc_gone" })
    render(<Host />)
    await screen.findByText("接続はまだありません")
    await user.click(screen.getByRole("button", { name: "追加する" }))
    await screen.findByText("clc_gone")
    await user.click(screen.getByRole("button", { name: "閉じる" }))
    expect(screen.queryByText("clc_gone")).not.toBeInTheDocument()
    expect(localStorage.length).toBe(0)
  })
})
