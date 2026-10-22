import { apiBaseURL } from "./health"

export type CreateSnapshotResponse = {
  snapshot_id: string
  warning: string
}

export type SnapshotSummary = {
  ID: string
  HTMLURL: string
  Owner: string
  Repo: string
  PRNumber: number
  Title: string
  AuthorLogin: string
  State: string
  CreatedAt: string
}

export async function createSnapshot(
    url: string,
    file: File | null,
  ): Promise<CreateSnapshotResponse> {
    const form = new FormData()
    form.append("url", url)
  
    if (file !== null) {
      form.append("file", file)
    }
  
    const response = await fetch(`${apiBaseURL()}/pr-snapshots`, {
      method: "POST",
      credentials: "include",
      body: form,
    })
  
    if (!response.ok) {
      const body = await response.json()
      throw new Error(body.error ?? "import_failed")
    }
  
    return (await response.json()) as CreateSnapshotResponse
  }

  export async function listSnapshots(): Promise<SnapshotSummary[]> {
    const response = await fetch(`${apiBaseURL()}/pr-snapshots`, {
      credentials: "include",
    })
  
    if (!response.ok) {
      const body = await response.json()
      throw new Error(body.error ?? "list_failed")
    }
  
    return (await response.json()) as SnapshotSummary[]
  }