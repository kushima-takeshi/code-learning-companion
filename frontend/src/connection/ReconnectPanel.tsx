import type { Reconnection } from "../api/connectionTokens"
import {
  canRestartReconnect,
  formatReconnectDeadline,
  isActiveReconnect,
  reconnectStateLabel,
  visibleSecret,
} from "./reconnect"

export type ReconnectBusy = "start" | "status" | "complete" | "restart" | null

function SecretBox({
  secret,
  busy,
  copyMessage,
  copyError,
  onCopy,
}: {
  secret: string
  busy: boolean
  copyMessage: string | null
  copyError: string | null
  onCopy: () => void
}) {
  return (
    <div className="reconnect-secret">
      <code>{secret}</code>
      <button type="button" onClick={onCopy} disabled={busy}>
        コピーする
      </button>
      {copyMessage && <p role="status">{copyMessage}</p>}
      {copyError && <p role="alert">{copyError}</p>}
    </div>
  )
}

export function ReconnectPanel({
  reconnection,
  secret,
  busy,
  error,
  copyMessage,
  copyError,
  onCheck,
  onComplete,
  onRestart,
  onStartAgain,
  onClose,
  onCopy,
}: {
  reconnection: Reconnection
  secret: string | null
  busy: ReconnectBusy
  error: string | null
  copyMessage: string | null
  copyError: string | null
  onCheck: () => void
  onComplete: () => void
  onRestart: () => void
  onStartAgain: () => void
  onClose: () => void
  onCopy: () => void
}) {
  const state = reconnection.state
  const label = reconnectStateLabel(state)
  const secretText = visibleSecret(state, secret)
  const locked = busy !== null
  const showHoldNote =
    state === "pending" || state === "confirmed" || state === "unavailable"

  return (
    <div
      id="reconnect-panel"
      className="reconnect-panel"
      role="region"
      aria-label="この接続を再接続する"
    >
      <div className="reconnect-panel-head">
        <h3>この接続を再接続する</h3>
        <button type="button" onClick={onClose} disabled={locked}>
          閉じる
        </button>
      </div>
      <p>
        再接続の期限：{formatReconnectDeadline(reconnection.expires_at)}
        （開始から24時間）
      </p>
      {showHoldNote && (
        <p>
          接続確認後に「置き換えを完了する」を押すまで、この処理で旧トークンを失効させません。
        </p>
      )}
      {error && <p role="alert">{error}</p>}

      {isActiveReconnect(reconnection) ? (
        <ol className="reconnect-steps">
          <li>
            <h4>新しいトークンをコピーする</h4>
            {secretText !== null ? (
              <>
                <SecretBox
                  secret={secretText}
                  busy={locked}
                  copyMessage={copyMessage}
                  copyError={copyError}
                  onCopy={onCopy}
                />
                <p>
                  この文字列は発行直後の今回だけ表示します。再読み込み後や、この欄を閉じたあとは再表示できません。
                </p>
              </>
            ) : (
              <p>
                トークンの文字列は発行直後の今回だけ表示します。再表示はできません。設定できないときは、やり直してください。
              </p>
            )}
          </li>
          <li>
            <h4>AIツールの設定を差し替え、学習記録の一覧を取得する</h4>
            <p>
              新しいトークンを接続設定へ入れて、一覧取得を試してください。記録が0件でも成功です。
            </p>
            <p>トークンはチャットや学習内容に貼り付けないでください。</p>
          </li>
          <li>
            <h4>Web画面で接続状態を確認する</h4>
            <button type="button" onClick={onCheck} disabled={locked}>
              {busy === "status" ? "確認しています" : "状態を確認する"}
            </button>
            <p
              role="status"
              className={
                state === "confirmed"
                  ? "reconnect-state is-confirmed"
                  : "reconnect-state is-pending"
              }
            >
              現在：{label}
            </p>
          </li>
          <li>
            <h4>確認済みになったら、置き換えを完了する</h4>
            {state === "confirmed" ? (
              <div className="reconnect-ready">
                <p className="reconnect-ready-title">接続確認済み（confirmed）</p>
                <p>新しいトークンで一覧取得に成功しました。</p>
                <p>完了すると、この接続の旧トークンだけを失効します。</p>
                <button
                  type="button"
                  className="primary"
                  onClick={onComplete}
                  disabled={locked}
                >
                  {busy === "complete" ? "完了しています" : "置き換えを完了する"}
                </button>
              </div>
            ) : (
              <p>接続確認が済むと、ここに「置き換えを完了する」を出します。</p>
            )}
          </li>
        </ol>
      ) : (
        <div className="reconnect-terminal">
          <p role="status" className="reconnect-state">
            現在：{label}
          </p>
          {state === "completed" && (
            <p>
              置き換えを完了しました。この接続の旧トークンだけを失効しました。新しいトークンは一覧に出ます。
            </p>
          )}
          {state === "expired" && (
            <>
              <p>
                開始から24時間を過ぎました。この操作の新しいトークンは使えません。旧トークンの期限と失効は変えていません。
              </p>
              <button
                type="button"
                className="primary"
                onClick={onStartAgain}
                disabled={locked}
              >
                {busy === "start" ? "始めています" : "改めて開始する"}
              </button>
            </>
          )}
          {state === "superseded" && (
            <>
              <p>
                この再接続はやり直されました。今の操作を開き直してください。トークンの文字列は再表示しません。
              </p>
              <button type="button" onClick={onStartAgain} disabled={locked}>
                {busy === "start" ? "始めています" : "今の再接続を開く"}
              </button>
            </>
          )}
          {state === "unavailable" && (
            <p>
              期限内ですが、新しいトークンは失効済みか期限切れです。旧トークンはこの処理では失効していません。やり直すと発行し直せます。
            </p>
          )}
          {state !== "completed" &&
            state !== "expired" &&
            state !== "superseded" &&
            state !== "unavailable" && (
              <p>
                この状態では、置き換えを完了できません。状態を確認するか、欄を閉じてください。
              </p>
            )}
          {secretText !== null && (
            <>
              <SecretBox
                secret={secretText}
                busy={locked}
                copyMessage={copyMessage}
                copyError={copyError}
                onCopy={onCopy}
              />
              <p>この欄を閉じたあとは、トークンの文字列を再表示できません。</p>
            </>
          )}
          <button type="button" onClick={onCheck} disabled={locked}>
            {busy === "status" ? "確認しています" : "状態を確認する"}
          </button>
        </div>
      )}

      {canRestartReconnect(state) && (
        <div className="reconnect-footer">
          <button
            type="button"
            className="reconnect-link"
            onClick={onRestart}
            disabled={locked}
          >
            {busy === "restart" ? "やり直しています" : "やり直す"}
          </button>
          <p>
            原本を失ったときなど。未完了の新しいトークンを失効し、新しく発行します。
          </p>
        </div>
      )}
    </div>
  )
}
