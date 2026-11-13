// Wails 3.1.0's Windows common-file-dialog returns this exact sentinel when
// ERROR_CANCELLED is reported. Other failures must retain their original error.
export async function fileDialogResult<T>(show: () => PromiseLike<T>, cancelled: T): Promise<T> {
  try {
    return await show();
  } catch (error) {
    const message = error instanceof Error ? error.message : error;
    if (message === "cancelled by user") return cancelled;
    throw error;
  }
}
