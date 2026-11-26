import { useEffect, useRef, useState, type MouseEventHandler } from "react"
import {
  getLearningRecord,
  listDueReviews,
  reviewResults,
  saveReviewResult,
  type DueReview,
  type LearningRecordDetail,
  type ReviewResult,
} from "../api/learningRecords"
import "./LearningRecordDetailSection.css"

export function LearningRecordDetailSection({
  encodedRecordId,
  onNavigate,
  onReviewSaved,
  onSessionExpired,
}: {
  encodedRecordId: string
  onNavigate: MouseEventHandler<HTMLAnchorElement>
  onReviewSaved: () => void
  onSessionExpired: () => void
}) {
  const [result, setResult] = useState<LearningRecordDetail | "loading" | "not-found" | "failed">("loading")
  const [dueReview, setDueReview] = useState<DueReview | "failed" | undefined>()
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)
  const [copyState, setCopyState] = useState<"copying" | "copied" | "failed">()
  const saveRequest = useRef<AbortController | null>(null)

  useEffect(() => {
    const controller = new AbortController()

    async function loadRecord() {
      try {
        const record = await getLearningRecord(encodedRecordId, controller.signal)
        if (controller.signal.aborted) return

        if (record === null) {
          onSessionExpired()
          return
        }
        setResult(record)
      } catch {
        if (!controller.signal.aborted) setResult("failed")
      }
    }

    async function loadDueReview() {
      try {
        const records = await listDueReviews(controller.signal)
        if (controller.signal.aborted) return

        if (records === null) {
          onSessionExpired()
          return
        }
        const recordId = decodeURIComponent(encodedRecordId)
        setDueReview(records.find((record) => record.id === recordId))
      } catch {
        if (!controller.signal.aborted) setDueReview("failed")
      }
    }

    void loadRecord()
    void loadDueReview()
    return () => {
      controller.abort()
      saveRequest.current?.abort()
    }
  }, [encodedRecordId, onSessionExpired])

  async function handleCopyResumeRequest(recordId: string) {
    setCopyState("copying")
    try {
      await navigator.clipboard.writeText(
        `get_learning_record で記録 ${recordId} を取得し、その復習を進めてください。`,
      )
      setCopyState("copied")
    } catch {
      setCopyState("failed")
    }
  }

  async function handleSaveReview(reviewResult: ReviewResult) {
    if (typeof dueReview !== "object" || saveRequest.current) return

    const controller = new AbortController()
    saveRequest.current = controller
    setSaving(true)
    setSaveError(null)

    try {
      const outcome = await saveReviewResult(
        encodedRecordId,
        { result: reviewResult, stage: dueReview.stage },
        controller.signal,
      )
      if (controller.signal.aborted) return

      if (outcome === "saved") {
        onReviewSaved()
      } else if (outcome === "unauthorized") {
        onSessionExpired()
      } else {
        setSaveError("復習の状態が変わりました。詳細を開き直してください。")
      }
    } catch {
      if (!controller.signal.aborted) setSaveError("復習の結果を保存できませんでした")
    } finally {
      saveRequest.current = null
      if (!controller.signal.aborted) setSaving(false)
    }
  }

  return (
    <section aria-labelledby="learning-record-detail-heading" className="learning-record-detail">
      <a href="/" onClick={onNavigate}>学習履歴へ戻る</a>
      <h2 id="learning-record-detail-heading">学習記録の詳細</h2>

      {result === "loading" && <p role="status">学習記録を読み込んでいます</p>}
      {result === "not-found" && <p role="alert">学習記録が見つかりませんでした</p>}
      {result === "failed" && <p role="alert">学習記録を取得できませんでした</p>}
      {typeof result === "object" && (
        <>
          <h3>{result.theme}</h3>
          {result.repository && <p>リポジトリ：{result.repository}</p>}
          <p>
            保存日時：
            <time dateTime={result.created_at}>
              {new Date(result.created_at).toLocaleString()}
            </time>
          </p>
          {dueReview === "failed" && <p role="alert">復習期限の一覧を取得できませんでした</p>}
          {typeof dueReview === "object" && (
            <section aria-labelledby="current-review-heading" className="current-review">
              <h4 id="current-review-heading">復習 {dueReview.stage}回目</h4>
              <p>
                期限：
                <time dateTime={dueReview.due_at}>{new Date(dueReview.due_at).toLocaleString()}</time>
              </p>
              <p>結果を保存しても復習期限は変わりません。</p>
              <div className="review-results">
                {reviewResults.map((reviewResult) => (
                  <button
                    key={reviewResult}
                    type="button"
                    disabled={saving}
                    onClick={() => void handleSaveReview(reviewResult)}
                  >
                    {reviewResult}
                  </button>
                ))}
              </div>
              {saveError && <p role="alert">{saveError}</p>}
              <button
                className="resume-copy-button"
                type="button"
                disabled={copyState === "copying"}
                onClick={() => void handleCopyResumeRequest(dueReview.id)}
              >
                再開用の依頼文をコピー
              </button>
              {copyState === "copied" && <p role="status">コピーしました</p>}
              {copyState === "failed" && <p role="alert">コピーできませんでした</p>}
            </section>
          )}
          {result.questions.map(({ prompt, answer, feedback }, index) => (
            <section key={index} aria-labelledby={`question-${index}`}>
              <h4 id={`question-${index}`}>問題 {index + 1}</h4>
              <p className="learning-record-text">{prompt}</p>
              <dl>
                <dt>回答</dt>
                <dd className="learning-record-text">{answer.text}</dd>
                {answer.code && (
                  <>
                    <dt>回答コード</dt>
                    <dd><pre><code>{answer.code}</code></pre></dd>
                  </>
                )}
                <dt>ヒント利用</dt>
                <dd>{answer.hint_used === null ? "記録なし" : answer.hint_used ? "あり" : "なし"}</dd>
                <dt>理解度</dt>
                <dd>{feedback.understanding}</dd>
                <dt>解説</dt>
                <dd className="learning-record-text">{feedback.explanation}</dd>
              </dl>
            </section>
          ))}
        </>
      )}
    </section>
  )
}
