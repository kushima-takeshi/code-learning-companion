import { apiBaseURL } from "./health"

export type CurrentUser = {
  id: string
  github_login: string
  display_name: string | null
  avatar_url: string | null
}

export async function fetchMe(): Promise<CurrentUser | null> {
  const response = await fetch(`${apiBaseURL()}/auth/me`, {
    credentials: "include",
  })

  if (response.status === 401) {
    return null
  }

  if (!response.ok) {
    throw new Error(`failed to fetch current user: ${response.status}`)
  }

  return (await response.json()) as CurrentUser
}

export async function logout(): Promise<void> {
  const response = await fetch(`${apiBaseURL()}/auth/logout`, {
    method: "POST",
    credentials: "include",
  })

  if (!response.ok) {
    throw new Error(`failed to logout: ${response.status}`)
  }
}
