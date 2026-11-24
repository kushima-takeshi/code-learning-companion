import { useEffect, useState, type MouseEventHandler } from "react"
import { getLearningRecord, type LearningRecordDetail } from "../api/learningRecords"
import "./LearningRecordDetailSection.css"

export function LearningRecordDetailSection({
  encodedRecordId,
  onNavigate,
  onSessionExpired,
}: {
  encodedRecordId: string
  onNavigate: MouseEventHandler<HTMLAnchorElement>
  onSessionExpired: () => void
}) {
  const [result, setResult] = useState<LearningRecordDetail | "loading" | "not-found" | "failed">("loading")

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

    void loadRecord()
    return () => controller.abort()
  }, [encodedRecordId, onSessionExpired])

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
