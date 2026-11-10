import {useEffect,useState} from "react";
import type {HttpCaptureTransactionRow,HttpCaptureFinding} from "@/bridge/types";
import {rowHints,findingRequestIds,statusHelpKey,type HttpAnnotation} from "@/state/httpAnnotations";
import {useI18n} from "@/i18n/I18nProvider";
export function HttpRowNotes({row,findings,disabled,onSave}:{row:HttpCaptureTransactionRow;findings:HttpCaptureFinding[];disabled:boolean;onSave:(value:HttpAnnotation)=>Promise<void>}):React.JSX.Element{
 const {t}=useI18n();const [comment,setComment]=useState(row.comment??"");useEffect(()=>setComment(row.comment??""),[row.id,row.comment]);
 return <div className="space-y-2 text-xs"><p className="font-semibold">{row.status} {row.status_text}</p><p>{t(statusHelpKey(row.status))}</p>{row.error&&<p role="status">{row.error}</p>}
 <ul>{rowHints(row).map(hint=><li key={hint.code}><code>{hint.code}</code> · {t(hint.key)}</li>)}</ul>
 {findings.filter(finding=>findingRequestIds(finding).includes(row.id)).map((finding,index)=><p key={index}><code>{finding.code}</code> · {finding.message}</p>)}
 <label className="flex gap-2"><input type="checkbox" checked={Boolean(row.starred)} disabled={disabled} onChange={event=>void onSave({starred:event.target.checked,comment:row.comment??""})}/>{t("httpNotesStar")}</label>
 <label className="block">{t("httpNotesComment")}<textarea aria-label={t("httpNotesComment")} value={comment} onChange={event=>setComment(event.target.value)} maxLength={1024} disabled={disabled} rows={3} className="block w-full rounded border p-2"/></label>
 <button type="button" disabled={disabled||comment===(row.comment??"")} onClick={()=>void onSave({starred:Boolean(row.starred),comment})}>{t("httpNotesSave")}</button><p className="text-muted-foreground">{t("httpNotesHint")}</p>
 </div>
}
