import { apiBaseURL } from "./health"

export type LearningRecordSummary = {
  id: string
  theme: string
  repository?: string
  created_at: string
}

export type LearningRecordPage = {
  records: LearningRecordSummary[]
  has_more: boolean
}

export async function listLearningRecords(
  offset: number,
  signal: AbortSignal,
): Promise<LearningRecordPage | null> {
  const query = offset === 0 ? "" : `?limit=20&offset=${offset}`
  const response = await fetch(`${apiBaseURL()}/web/learning-records${query}`, {
    credentials: "include",
    signal,
  })

  if (response.status === 401) {
    return null
  }
  if (!response.ok) {
    throw new Error(`failed to list learning records: ${response.status}`)
  }

  return (await response.json()) as LearningRecordPage
}
