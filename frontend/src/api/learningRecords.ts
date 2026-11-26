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

export const reviewResults = ["できた", "一部できた", "もう一度", "判定保留"] as const

export type ReviewResult = (typeof reviewResults)[number]

export async function saveReviewResult(
  encodedRecordId: string,
  review: { result: ReviewResult; stage: number },
  signal: AbortSignal,
): Promise<"saved" | "unauthorized" | "conflict"> {
  const response = await fetch(`${apiBaseURL()}/web/learning-records/${encodedRecordId}/reviews`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(review),
    signal,
  })

  if (response.status === 201) return "saved"
  if (response.status === 401) return "unauthorized"
  if (response.status === 409) return "conflict"
  throw new Error(`failed to save review result: ${response.status}`)
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
