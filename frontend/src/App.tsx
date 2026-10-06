import { useEffect, useState } from "react"
import { fetchHealth } from "./api/health"

function App() {
  const [health, setHealth] = useState("loading")

  useEffect(() => {
    let ignore = false

    fetchHealth()
      .then((body) => {
        if (!ignore) {
          setHealth(body.status)
        }
      })
      .catch((err: unknown) => {
        if (!ignore) {
          setHealth(err instanceof Error ? err.message : "request failed")
        }
      })

    return () => {
      ignore = true
    }
  }, [])

  return (
    <main style={{ padding: "2rem", fontFamily: "sans-serif" }}>
      <h1>Code Learning Companion</h1>
      <p>API health: {health}</p>
    </main>
  )
}

export default App