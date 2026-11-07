import { render, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import type { ConnectionToken, Reconnection } from "../api/connectionTokens"
import { ConnectionSection, type ConnectionApi } from "./ConnectionSection"

const now = new Date("2026-11-08T00:00:00.000Z")
const tokenId = "11111111-1111-1111-1111-111111111111"
const otherId = "22222222-2222-2222-2222-222222222222"

function token(overrides: Partial<ConnectionToken> = {}): ConnectionToken {
  return {
    id: tokenId,
    name: "codex",
    created_at: "2026-10-01T00:00:00.000Z",
    expires_at: "2027-02-04T00:00:00.000Z",
    revoked_at: null,
    ...overrides,
  }
}

function reconnection(overrides: Partial<Reconnection> = {}): Reconnection {
  return {
    id: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
    old_token_id: tokenId,
    new_token_id: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
    state: "pending",
    created_at: "2026-11-07T15:00:00.000Z",
    expires_at: "2026-11-08T15:00:00.000Z",
    confirmed_at: null,
    completed_at: null,
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

describe("再接続の画面", () => {
  let api: {
    list: ReturnType<typeof vi.fn<ConnectionApi["list"]>>
    create: ReturnType<typeof vi.fn<ConnectionApi["create"]>>
    rename: ReturnType<typeof vi.fn<ConnectionApi["rename"]>>
    revoke: ReturnType<typeof vi.fn<ConnectionApi["revoke"]>>
    startReconnect: ReturnType<typeof vi.fn<ConnectionApi["startReconnect"]>>
    getReconnect: ReturnType<typeof vi.fn<ConnectionApi["getReconnect"]>>
    completeReconnect: ReturnType<typeof vi.fn<ConnectionApi["completeReconnect"]>>
    restartReconnect: ReturnType<typeof vi.fn<ConnectionApi["restartReconnect"]>>
  }
  let confirmReconnect: ReturnType<typeof vi.fn<(message: string) => boolean>>
  let copyText: ReturnType<typeof vi.fn<(value: string) => Promise<void>>>

  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    api = {
      list: vi.fn(async () => [token()]),
      create: vi.fn(),
      rename: vi.fn(),
      revoke: vi.fn(async () => {}),
      startReconnect: vi.fn(),
      getReconnect: vi.fn(),
      completeReconnect: vi.fn(),
      restartReconnect: vi.fn(),
    }
    confirmReconnect = vi.fn(() => true)
    copyText = vi.fn(async () => {})
  })

  function renderSection() {
    return render(
      <ConnectionSection
        api={api}
        now={() => now}
        confirmReconnect={confirmReconnect}
        copyText={copyText}
      />,
    )
  }

  async function rowOf(name: string) {
    const label = await screen.findByText(name)
    const row = label.closest("li")
    if (row === null) {
      throw new Error(`row not found: ${name}`)
    }
    return row
  }

  it("開始すると対象の行の下に開き、原本は一度だけ、完了ボタンは出さない", async () => {
    const user = userEvent.setup()
    const setItem = vi.spyOn(Storage.prototype, "setItem")
    api.list.mockResolvedValue([token(), token({ id: otherId, name: "cursor" })])
    api.startReconnect.mockResolvedValue({
      reconnection: reconnection(),
      secret: "clc_new",
    })
    renderSection()

    const row = await rowOf("codex")
    await user.click(within(row).getByRole("button", { name: "再接続" }))

    const panel = await within(row).findByRole("region", {
      name: "この接続を再接続する",
    })
    expect(api.startReconnect).toHaveBeenCalledWith(tokenId)
    expect(api.getReconnect).not.toHaveBeenCalled()
    expect(api.completeReconnect).not.toHaveBeenCalled()
    expect(within(panel).getByText("clc_new")).toBeInTheDocument()
    expect(screen.getAllByText("clc_new")).toHaveLength(1)
    expect(within(panel).getByText(/2026\/11\/09 00:00:00 JST/)).toBeInTheDocument()
    expect(within(panel).getByText(/開始から24時間/)).toBeInTheDocument()
    expect(within(panel).getByText(/旧トークンを失効させません/)).toBeInTheDocument()
    expect(within(panel).getByText(/記録が0件でも成功です/)).toBeInTheDocument()
    expect(within(panel).getByText(/チャットや学習内容に貼り付けないでください/)).toBeInTheDocument()
    expect(within(panel).getByRole("status")).toHaveTextContent(
      "現在：接続確認待ち（pending）",
    )
    expect(
      within(panel).queryByRole("button", { name: "置き換えを完了する" }),
    ).not.toBeInTheDocument()
    expect(within(panel).getByRole("button", { name: "やり直す" })).toBeInTheDocument()
    expect(within(row).getByRole("button", { name: "再接続中" })).toHaveAttribute(
      "aria-expanded",
      "true",
    )
    expect(within(await rowOf("cursor")).queryByRole("region")).not.toBeInTheDocument()
    expect(screen.queryByText(/mcp|config/i)).not.toBeInTheDocument()
    expect(localStorage.length).toBe(0)
    expect(sessionStorage.length).toBe(0)
    expect(setItem).not.toHaveBeenCalled()

    await user.click(within(panel).getByRole("button", { name: "コピーする" }))
    expect(copyText).toHaveBeenCalledWith("clc_new")
    expect(within(panel).getByText("コピーしました")).toHaveAttribute("role", "status")

    await user.click(within(row).getByRole("button", { name: "再接続中" }))
    expect(screen.queryByText("clc_new")).not.toBeInTheDocument()
    expect(screen.queryByRole("region", { name: "この接続を再接続する" })).not.toBeInTheDocument()
    setItem.mockRestore()
  })

  it("状態を確認するまでGETせず、confirmedのときだけ完了でき、自動では完了しない", async () => {
    const user = userEvent.setup()
    const operation = reconnection()
    api.startReconnect.mockResolvedValue({ reconnection: operation, secret: "clc_new" })
    api.getReconnect
      .mockResolvedValueOnce(operation)
      .mockResolvedValueOnce(
        reconnection({
          state: "confirmed",
          confirmed_at: "2026-11-08T01:00:00.000Z",
        }),
      )
    api.completeReconnect.mockResolvedValue(
      reconnection({
        state: "completed",
        confirmed_at: "2026-11-08T01:00:00.000Z",
        completed_at: "2026-11-08T02:00:00.000Z",
      }),
    )
    api.list.mockImplementation(async () => {
      if (api.completeReconnect.mock.calls.length > 0) {
        return [
          token({ revoked_at: "2026-11-08T02:00:00.000Z" }),
          token({
            id: "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
            name: "codex-new",
          }),
        ]
      }
      return [token()]
    })
    renderSection()
    await user.click(await screen.findByRole("button", { name: "再接続" }))
    const panel = await screen.findByRole("region", { name: "この接続を再接続する" })
    expect(api.getReconnect).not.toHaveBeenCalled()

    await user.click(within(panel).getByRole("button", { name: "状態を確認する" }))
    expect(api.getReconnect).toHaveBeenCalledTimes(1)
    expect(api.getReconnect).toHaveBeenCalledWith(operation.id)
    expect(api.completeReconnect).not.toHaveBeenCalled()
    expect(
      within(panel).queryByRole("button", { name: "置き換えを完了する" }),
    ).not.toBeInTheDocument()

    await user.click(within(panel).getByRole("button", { name: "状態を確認する" }))
    expect(api.getReconnect).toHaveBeenCalledTimes(2)
    expect(
      await within(panel).findByRole("button", { name: "置き換えを完了する" }),
    ).toBeInTheDocument()
    expect(api.completeReconnect).not.toHaveBeenCalled()
    expect(within(panel).getByText("clc_new")).toBeInTheDocument()

    await user.click(within(panel).getByRole("button", { name: "置き換えを完了する" }))
    expect(api.completeReconnect).toHaveBeenCalledTimes(1)
    expect(api.completeReconnect).toHaveBeenCalledWith(operation.id)
    expect(await within(panel).findByText(/置き換えを完了しました/)).toBeInTheDocument()
    expect(
      within(panel).queryByRole("button", { name: "置き換えを完了する" }),
    ).not.toBeInTheDocument()
    expect(within(panel).queryByRole("button", { name: "やり直す" })).not.toBeInTheDocument()
    expect(within(panel).getByText("clc_new")).toBeInTheDocument()
    expect(await screen.findByText("codex-new")).toBeInTheDocument()
    expect(within(panel.closest("li")!).getByText("使えない")).toBeInTheDocument()
  })

  it("原本のない再開では再表示せず、やり直すは確認してから新しい原本に替える", async () => {
    const user = userEvent.setup()
    const operation = reconnection()
    api.startReconnect.mockResolvedValue({ reconnection: operation, secret: null })
    api.restartReconnect.mockResolvedValue({
      reconnection: reconnection({
        id: "cccccccc-cccc-cccc-cccc-cccccccccccc",
      }),
      secret: "clc_restart",
    })
    renderSection()
    await user.click(await screen.findByRole("button", { name: "再接続" }))
    const panel = await screen.findByRole("region", { name: "この接続を再接続する" })
    expect(within(panel).getByText(/再表示はできません/)).toBeInTheDocument()
    expect(within(panel).queryByRole("button", { name: "コピーする" })).not.toBeInTheDocument()

    confirmReconnect.mockReturnValueOnce(false)
    await user.click(within(panel).getByRole("button", { name: "やり直す" }))
    expect(confirmReconnect).toHaveBeenCalled()
    expect(String(confirmReconnect.mock.calls[0][0])).toContain("失効")
    expect(api.restartReconnect).not.toHaveBeenCalled()

    confirmReconnect.mockReturnValueOnce(true)
    await user.click(within(panel).getByRole("button", { name: "やり直す" }))
    expect(api.restartReconnect).toHaveBeenCalledWith(operation.id)
    expect(await within(panel).findByText("clc_restart")).toBeInTheDocument()
    expect(localStorage.length).toBe(0)
    expect(sessionStorage.length).toBe(0)
  })

  it("使えない接続への開始は作り直す案内だけして、原本も新しいトークンも出さない", async () => {
    const user = userEvent.setup()
    api.startReconnect.mockRejectedValue(new Error("token_unavailable"))
    renderSection()
    await user.click(await screen.findByRole("button", { name: "再接続" }))

    const alert = await screen.findByRole("alert")
    expect(alert).toHaveTextContent("再接続を始められません")
    expect(alert).toHaveTextContent("追加するまで新しいトークンは作りません")
    expect(screen.getByLabelText("接続名")).toHaveValue("codex")
    expect(screen.getByLabelText("接続名")).toHaveFocus()
    expect(api.create).not.toHaveBeenCalled()
    expect(screen.queryByRole("region", { name: "この接続を再接続する" })).not.toBeInTheDocument()
  })

  it("expired、superseded、unavailable、完了済みの再開を状態に合わせて出す", async () => {
    const user = userEvent.setup()
    api.startReconnect.mockResolvedValueOnce({
      reconnection: reconnection({ state: "expired" }),
      secret: "clc_hidden",
    })
    renderSection()
    await user.click(await screen.findByRole("button", { name: "再接続" }))
    let panel = await screen.findByRole("region", { name: "この接続を再接続する" })
    expect(within(panel).getByText(/24時間を過ぎました/)).toBeInTheDocument()
    expect(within(panel).queryByText("clc_hidden")).not.toBeInTheDocument()
    expect(within(panel).queryByRole("button", { name: "やり直す" })).not.toBeInTheDocument()
    expect(
      within(panel).queryByRole("button", { name: "置き換えを完了する" }),
    ).not.toBeInTheDocument()

    api.startReconnect.mockResolvedValueOnce({
      reconnection: reconnection(),
      secret: "clc_again",
    })
    await user.click(within(panel).getByRole("button", { name: "改めて開始する" }))
    expect(api.startReconnect).toHaveBeenCalledTimes(2)
    expect(await within(panel).findByText("clc_again")).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "再接続中" }))
    api.startReconnect.mockResolvedValueOnce({
      reconnection: reconnection({ state: "superseded" }),
      secret: null,
    })
    await user.click(screen.getByRole("button", { name: "再接続" }))
    panel = await screen.findByRole("region", { name: "この接続を再接続する" })
    expect(within(panel).getByText(/やり直されました/)).toBeInTheDocument()
    api.startReconnect.mockResolvedValueOnce({
      reconnection: reconnection({ state: "pending" }),
      secret: null,
    })
    await user.click(within(panel).getByRole("button", { name: "今の再接続を開く" }))
    expect(await within(panel).findByText(/再表示はできません/)).toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "再接続中" }))
    api.startReconnect.mockResolvedValueOnce({
      reconnection: reconnection({ state: "unavailable" }),
      secret: "clc_hidden",
    })
    await user.click(screen.getByRole("button", { name: "再接続" }))
    panel = await screen.findByRole("region", { name: "この接続を再接続する" })
    expect(within(panel).getByText(/新しいトークンは失効済みか期限切れです/)).toBeInTheDocument()
    expect(within(panel).queryByText("clc_hidden")).not.toBeInTheDocument()
    expect(within(panel).getByRole("button", { name: "やり直す" })).toBeInTheDocument()
    expect(
      within(panel).queryByRole("button", { name: "置き換えを完了する" }),
    ).not.toBeInTheDocument()

    await user.click(screen.getByRole("button", { name: "再接続中" }))
    api.startReconnect.mockResolvedValueOnce({
      reconnection: reconnection({ state: "completed" }),
      secret: null,
    })
    await user.click(screen.getByRole("button", { name: "再接続" }))
    panel = await screen.findByRole("region", { name: "この接続を再接続する" })
    expect(within(panel).getByText(/置き換えを完了しました/)).toBeInTheDocument()
    expect(within(panel).queryByRole("button", { name: "やり直す" })).not.toBeInTheDocument()
    expect(api.create).not.toHaveBeenCalled()
  })

  it("完了が409のときは状態を読み直し、完了ボタンを残さない", async () => {
    const user = userEvent.setup()
    api.startReconnect.mockResolvedValue({
      reconnection: reconnection({
        state: "confirmed",
        confirmed_at: "2026-11-08T01:00:00.000Z",
      }),
      secret: null,
    })
    api.completeReconnect.mockRejectedValue(new Error("reconnection_conflict"))
    api.getReconnect.mockResolvedValue(reconnection({ state: "expired" }))
    renderSection()
    await user.click(await screen.findByRole("button", { name: "再接続" }))
    const panel = await screen.findByRole("region", { name: "この接続を再接続する" })
    expect(api.getReconnect).not.toHaveBeenCalled()
    await user.click(within(panel).getByRole("button", { name: "置き換えを完了する" }))

    expect(await within(panel).findByRole("alert")).toHaveTextContent(
      "この状態では置き換えを完了できません",
    )
    expect(api.getReconnect).toHaveBeenCalledTimes(1)
    expect(within(panel).getByText(/24時間を過ぎました/)).toBeInTheDocument()
    expect(
      within(panel).queryByRole("button", { name: "置き換えを完了する" }),
    ).not.toBeInTheDocument()
  })

  it("別の行を開くと、前の原本は捨ててその行だけを開く", async () => {
    const user = userEvent.setup()
    api.list.mockResolvedValue([token(), token({ id: otherId, name: "cursor" })])
    api.startReconnect
      .mockResolvedValueOnce({ reconnection: reconnection(), secret: "clc_first" })
      .mockResolvedValueOnce({
        reconnection: reconnection({
          id: "dddddddd-dddd-dddd-dddd-dddddddddddd",
          old_token_id: otherId,
        }),
        secret: "clc_second",
      })
    renderSection()
    await user.click(
      within(await rowOf("codex")).getByRole("button", { name: "再接続" }),
    )
    expect(await screen.findByText("clc_first")).toBeInTheDocument()
    await user.click(
      within(await rowOf("cursor")).getByRole("button", { name: "再接続" }),
    )
    expect(await screen.findByText("clc_second")).toBeInTheDocument()
    expect(screen.queryByText("clc_first")).not.toBeInTheDocument()
    expect(screen.getAllByRole("region", { name: "この接続を再接続する" })).toHaveLength(1)
    expect(api.startReconnect).toHaveBeenNthCalledWith(2, otherId)
  })

  it("開始の途中では再接続のボタンを押せない", async () => {
    const user = userEvent.setup()
    const pending = deferred<{ reconnection: Reconnection; secret: string | null }>()
    api.startReconnect.mockReturnValue(pending.promise)
    renderSection()
    await user.click(await screen.findByRole("button", { name: "再接続" }))
    expect(screen.getByRole("button", { name: "始めています" })).toBeDisabled()

    pending.resolve({ reconnection: reconnection(), secret: "clc_done" })
    expect(await screen.findByText("clc_done")).toBeInTheDocument()
  })

  it("欄を閉じると原本は残らない", async () => {
    const user = userEvent.setup()
    function Host() {
      const [open, setOpen] = useState(true)
      return open ? (
        <>
          <button type="button" onClick={() => setOpen(false)}>
            画面を閉じる
          </button>
          <ConnectionSection
            api={api}
            now={() => now}
            confirmReconnect={confirmReconnect}
            copyText={copyText}
          />
        </>
      ) : (
        <p>閉じた</p>
      )
    }
    api.startReconnect.mockResolvedValue({
      reconnection: reconnection(),
      secret: "clc_gone",
    })
    render(<Host />)
    await user.click(await screen.findByRole("button", { name: "再接続" }))
    await screen.findByText("clc_gone")
    await user.click(screen.getByRole("button", { name: "画面を閉じる" }))
    expect(screen.queryByText("clc_gone")).not.toBeInTheDocument()
    expect(localStorage.length).toBe(0)
    expect(sessionStorage.length).toBe(0)
  })
})
