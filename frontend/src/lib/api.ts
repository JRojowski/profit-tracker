export type Health = {
  status: string
}

export async function fetchHealth(signal?: AbortSignal): Promise<Health> {
  const response = await fetch("/api/health", { signal })
  if (!response.ok) {
    throw new Error(`health check failed: HTTP ${response.status}`)
  }
  return (await response.json()) as Health
}
