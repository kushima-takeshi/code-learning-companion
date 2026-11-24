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

export type DueReview = Pick<LearningRecordSummary, "id" | "theme" | "repository"> & {
  stage: number
  due_at: string
}

export async function listDueReviews(signal: AbortSignal): Promise<DueReview[] | null> {
  const response = await fetch(`${apiBaseURL()}/web/reviews/due`, {
    credentials: "include",
    signal,
  })

  if (response.status === 401) return null
  if (!response.ok) {
    throw new Error(`failed to list due reviews: ${response.status}`)
  }

  const body = (await response.json()) as { records: DueReview[] }
  return body.records
}

export type LearningRecordDetail = LearningRecordSummary & {
  request_id: string
  questions: {
    type: string
    prompt: string
    answer: { text: string; hint_used: boolean | null; code?: string }
    feedback: { understanding: string; explanation: string }
  }[]
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

export async function getLearningRecord(
  encodedRecordId: string,
  signal: AbortSignal,
): Promise<LearningRecordDetail | "not-found" | null> {
  const response = await fetch(`${apiBaseURL()}/web/learning-records/${encodedRecordId}`, {
    credentials: "include",
    signal,
  })

  if (response.status === 401) return null
  if (response.status === 404) return "not-found"
  if (!response.ok) {
    throw new Error(`failed to get learning record: ${response.status}`)
  }

  return (await response.json()) as LearningRecordDetail
}
