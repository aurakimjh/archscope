import type {HttpCaptureTransactionRow,HttpCaptureFinding} from "../bridge/types";
import type {MessageKey} from "../i18n/messages";
export type HttpAnnotation={starred:boolean;comment:string};
export const MAX_ANNOTATION_COMMENT_BYTES=4096;
export function annotationCommentBytes(comment:string):number{return new TextEncoder().encode(comment).length;}
export function rowHints(row:HttpCaptureTransactionRow):{code:string;key:MessageKey}[]{
 const out:{code:string;key:MessageKey}[]=[];const add=(condition:boolean,code:string,key:MessageKey)=>{if(condition)out.push({code,key})};
 add(row.status>=400,"HTTP_ERROR","httpHintHttpError");add(["failed","aborted"].includes(row.state),"INCOMPLETE","httpHintIncomplete");add(["request_sent","receiving"].includes(row.state),"PENDING","httpHintPending");add([row.request,row.response].some(m=>m.bodyStorage==="omitted"||m.bodyStorage==="export_omitted"),"BODY_OMITTED","httpHintOmitted");add([row.request,row.response].some(m=>m.bodyStorage==="truncated"),"BODY_TRUNCATED","httpHintTruncated");add([row.request,row.response].some(m=>m.redacted||(m.headers??[]).some(h=>h.redacted)),"REDACTED","httpHintRedacted");add(!row.process||row.process.attribution!=="confirmed","PROCESS_UNKNOWN","httpHintProcess");add(Boolean(row.informational_truncated),"INTERIM_TRUNCATED","httpCaptureInterimTruncated");return out;
}
export function findingRequestIds(finding:HttpCaptureFinding):string[]{const evidence=finding.evidence??{};const candidates=[evidence.transaction_id,evidence.transactionId,evidence.request_id,...(Array.isArray(evidence.transaction_ids)?evidence.transaction_ids:[])];return [...new Set(candidates.filter((id):id is string=>typeof id==="string"&&id!==""))]}
export function statusHelpKey(status:number):MessageKey{if(status<100)return "httpStatusPending";if(status<200)return "httpStatus1xx";if(status<300)return "httpStatus2xx";if(status<400)return "httpStatus3xx";if(status===401||status===403)return "httpStatusAuth";if(status===429)return "httpStatusRate";if(status<500)return "httpStatus4xx";if(status===502||status===504)return "httpStatusGateway";return "httpStatus5xx"}
