import { useEffect, useState } from "react"
import {
  listLearningRecords,
  type LearningRecordSummary,
} from "../api/learningRecords"

function groupRecords(records: LearningRecordSummary[]) {
  const byRepository = new Map<string, LearningRecordSummary[]>()
  const withoutRepository: LearningRecordSummary[] = []

  for (const record of records) {
    if (!record.repository) {
      withoutRepository.push(record)
      continue
    }
    const group = byRepository.get(record.repository)
    if (group) {
      group.push(record)
    } else {
      byRepository.set(record.repository, [record])
    }
  }

  if (withoutRepository.length > 0) {
    byRepository.set("", withoutRepository)
  }
  return byRepository
}

export function LearningHistorySection({
  onSessionExpired,
}: {
  onSessionExpired: () => void
}) {
  const [records, setRecords] = useState<LearningRecordSummary[]>([])
  const [request, setRequest] = useState({ offset: 0 })
  const [nextOffset, setNextOffset] = useState<number | null>(null)
  const [loading, setLoading] = useState(true)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    const controller = new AbortController()

    async function loadPage() {
      try {
        const page = await listLearningRecords(request.offset, controller.signal)
        if (controller.signal.aborted) return

        if (page === null) {
          onSessionExpired()
          return
        }

        setRecords((previous) => {
          const seen = new Set(previous.map((record) => record.id))
          const newRecords = page.records.filter((record) => !seen.has(record.id))
          return [...previous, ...newRecords]
        })
        setNextOffset(page.has_more ? request.offset + page.records.length : null)
      } catch {
        if (!controller.signal.aborted) setFailed(true)
      } finally {
        if (!controller.signal.aborted) setLoading(false)
      }
    }

    void loadPage()
    return () => controller.abort()
  }, [request, onSessionExpired])

  function requestPage(offset: number) {
    setLoading(true)
    setFailed(false)
    setRequest({ offset })
  }

  return (
    <section aria-labelledby="learning-history-heading">
      <h2 id="learning-history-heading">学習履歴</h2>

      {[...groupRecords(records)].map(([repository, group]) => (
        <section key={repository}>
          <h3>{repository || "リポジトリなし"}</h3>
          <ul>
            {group.map((record) => (
              <li key={record.id}>
                <p>{record.theme}</p>
                <p>
                  保存日時：
                  <time dateTime={record.created_at}>
                    {new Date(record.created_at).toLocaleString()}
                  </time>
                </p>
              </li>
            ))}
          </ul>
        </section>
      ))}

      {loading && <p role="status">学習履歴を読み込んでいます</p>}
      {failed && <p role="alert">学習履歴を取得できませんでした</p>}
      {!loading && !failed && records.length === 0 && (
        <p>学習記録はまだありません</p>
      )}

      {failed ? (
        <button type="button" onClick={() => requestPage(request.offset)}>
          再試行
        </button>
      ) : (
        nextOffset !== null && (
          <button
            type="button"
            disabled={loading}
            onClick={() => requestPage(nextOffset)}
          >
            さらに表示
          </button>
        )
      )}
    </section>
  )
}
