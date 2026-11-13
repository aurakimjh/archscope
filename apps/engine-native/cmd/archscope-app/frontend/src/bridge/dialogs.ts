import { Dialogs as NativeDialogs } from "@wailsio/runtime";
import { fileDialogResult } from "../state/fileDialog";

type OpenOptions = Parameters<typeof NativeDialogs.OpenFile>[0];
function openFile(options: OpenOptions & { AllowsMultipleSelection: true }): Promise<string[]>;
function openFile(options: OpenOptions & { AllowsMultipleSelection?: false }): Promise<string>;
function openFile(options: OpenOptions): Promise<string | string[]>;
function openFile(options: OpenOptions = {}): Promise<string | string[]> {
  return fileDialogResult(() => NativeDialogs.OpenFile(options), options.AllowsMultipleSelection ? [] : "");
}

// Keep cancellation identical to Wails' documented empty selection on every
// page, before an import/export error handler can misclassify it as a failure.
export const Dialogs = {
  ...NativeDialogs,
  OpenFile: openFile,
  SaveFile: (options: Parameters<typeof NativeDialogs.SaveFile>[0] = {}) =>
    fileDialogResult(() => NativeDialogs.SaveFile(options), ""),
};
