/**
 * Message of a failed API response: the `error` the API sends in its JSON
 * body (`{"error": "network not found"}`), or the HTTP status when the body
 * carries none.
 */
export async function apiErrorMessage(res: Response): Promise<string> {
  try {
    const body: unknown = await res.json();
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string' && body.error) {
      return body.error;
    }
  } catch {
    // Not JSON: fall back to the status.
  }
  return `HTTP ${res.status}`;
}
