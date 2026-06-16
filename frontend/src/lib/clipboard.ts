// copyText writes text to the system clipboard. It resolves to true on
// success and false on any failure (no clipboard API, denied permission,
// unfocused document) instead of leaking an unhandled promise rejection.
// Callers use the boolean to decide whether to confirm the copy in the UI.
export async function copyText(text: string): Promise<boolean> {
  if (typeof navigator === 'undefined' || !navigator.clipboard) {
    return false;
  }
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return false;
  }
}
