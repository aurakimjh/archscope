// Wails 3.1.0 wraps ERROR_CANCELLED differently for single/multiple OpenFile
// and SaveFile. Match only these complete messages, preserving other failures.
const cancelledMessages = new Set([
  "cancelled by user",
  "Invalid dialog call: Dialog.OpenFile failed, error getting selection: cancelled by user",
  "Invalid dialog call: Dialog.OpenFile failed: error getting selection: cancelled by user",
  "Invalid dialog call: Dialog.SaveFile failed: error getting selection: cancelled by user",
]);
export async function fileDialogResult<T>(show: () => PromiseLike<T>, cancelled: T): Promise<T> {
  try {
    return await show();
  } catch (error) {
    const message = error instanceof Error ? error.message : error;
    if (typeof message === "string" && cancelledMessages.has(message)) return cancelled;
    throw error;
  }
}
