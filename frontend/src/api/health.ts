export type HealthResponse = {
  status: string
}

function apiBaseURL(): string {
  const configured = import.meta.env.VITE_API_URL
  if (typeof configured === "string" && configured.length > 0) {
    return configured.replace(/\/$/, "")
  }
  return "http://localhost:8080"
}

export async function fetchHealth(): Promise<HealthResponse> {
  const response = await fetch(`${apiBaseURL()}/health`)
  if (!response.ok) {
    throw new Error(`health check failed: ${response.status}`)
  }
  return (await response.json()) as HealthResponse
}