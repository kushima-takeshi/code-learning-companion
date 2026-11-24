import { useEffect, useState, type MouseEventHandler } from "react"
import { listDueReviews, type DueReview } from "../api/learningRecords"
import "./DueReviewsSection.css"

export function DueReviewsSection({
  onNavigate,
  onSessionExpired,
}: {
  onNavigate: MouseEventHandler<HTMLAnchorElement>
  onSessionExpired: () => void
}) {
  const [result, setResult] = useState<DueReview[] | "loading" | "failed">("loading")

  useEffect(() => {
    if (result !== "loading") return
    const controller = new AbortController()

    async function loadDueReviews() {
      try {
        const records = await listDueReviews(controller.signal)
        if (controller.signal.aborted) return

        if (records === null) {
          onSessionExpired()
          return
        }
        setResult(records)
      } catch {
        if (!controller.signal.aborted) setResult("failed")
      }
    }

    void loadDueReviews()
    return () => controller.abort()
  }, [result, onSessionExpired])

  return (
    <section aria-labelledby="due-reviews-heading" className="due-reviews">
      <h2 id="due-reviews-heading">復習期限が来ている記録</h2>

      {result === "loading" && <p role="status">復習期限の一覧を読み込んでいます</p>}
      {result === "failed" && (
        <>
          <p role="alert">復習期限の一覧を取得できませんでした</p>
          <button type="button" onClick={() => setResult("loading")}>再試行</button>
        </>
      )}
      {Array.isArray(result) && (
        result.length === 0 ? <p>復習期限が来ている記録はありません</p> : (
          <ol className="due-reviews-list">
            {result.map((record) => (
              <li key={record.id}>
                <a href={`/learning-records/${encodeURIComponent(record.id)}`} onClick={onNavigate}>
                  {record.theme}
                </a>
                {record.repository && <p>リポジトリ：{record.repository}</p>}
                <p className="due-review-timing">
                  <span>段階 {record.stage}</span>
                  <span>
                    期限：
                    <time dateTime={record.due_at}>
                      {new Date(record.due_at).toLocaleString()}
                    </time>
                  </span>
                </p>
              </li>
            ))}
          </ol>
        )
      )}
    </section>
  )
}
