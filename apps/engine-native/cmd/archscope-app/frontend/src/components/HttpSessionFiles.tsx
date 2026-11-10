import { Dialogs } from "@wailsio/runtime";
import { useState } from "react";
import { useI18n } from "@/i18n/I18nProvider";
import * as CaptureService from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/cmd/archscope-app/captureservice";
export type HttpFileSource = { sessionId?: string; path?: string };
const formats = {session:["ArchScope session","httpsession"],har:["HAR 1.2","har"],xml:["Full XML","xml"],xml_compact:["Compact XML","xml"],urls:["URL list","txt"],text:["Text","txt"],html:["HTML","html"],xls:["Excel 97–2003","xls"],responses:["Response previews","directory"]} as const;
export function HttpSessionFiles({source,ids,selectedId}: {source:HttpFileSource;ids:string[];selectedId:string|null}):React.JSX.Element {
 const {t}=useI18n();const [format,setFormat]=useState<keyof typeof formats>("session");const [scope,setScope]=useState("all");const [busy,setBusy]=useState(false);const [notice,setNotice]=useState("");const [error,setError]=useState("");
 const save=async()=>{setBusy(true);setError("");setNotice("");try{
  const picked=format==="responses"?await Dialogs.OpenFile({Title:t("httpFilesDestination"),CanChooseDirectories:true,CanChooseFiles:false,CanCreateDirectories:true}):await Dialogs.SaveFile({Title:t("httpFilesDestination"),Filename:`http-session.${formats[format][1]}`,Filters:[{DisplayName:formats[format][0],Pattern:`*.${formats[format][1]}`}]});
  const destination=Array.isArray(picked)?picked[0]:picked;if(!destination)return;
  const count=await CaptureService.ExportCaptureFile({source,destination,format,scope,ids:scope==="filtered"?ids:[],selectedId:selectedId??""});setNotice(`${count} · ${destination}`);
 }catch(caught){setError(String(caught))}finally{setBusy(false)}};
 return <div className="flex shrink-0 flex-wrap items-center gap-2 border-b px-2 py-1 text-xs">
  <span>{t("httpFilesTitle")}</span><select aria-label={t("httpFilesFormat")} value={format} onChange={e=>setFormat(e.target.value as keyof typeof formats)}>{Object.entries(formats).map(([key,value])=><option key={key} value={key}>{value[0]}</option>)}</select>
  <select aria-label={t("httpFilesScope")} value={scope} onChange={e=>setScope(e.target.value)}><option value="all">{t("httpFilesAll")}</option><option value="filtered">{t("httpFilesFiltered")} ({ids.length})</option></select>
  <button type="button" disabled={busy} onClick={()=>void save()}>{t("httpFilesSave")}</button><span className="text-muted-foreground">{t("httpFilesPrivacy")}</span>
  {notice&&<span role="status">{notice}</span>}{error&&<span role="alert" className="text-red-600">{error}</span>}
 </div>
}
