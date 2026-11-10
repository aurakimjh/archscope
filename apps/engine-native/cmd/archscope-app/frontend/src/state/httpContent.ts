import type {CaptureHTTPMessage,HttpCaptureTransactionRow} from "../bridge/types";
import {buildRawRequest,buildRawResponse} from "./httpAnalyzer.js";
export function cookieFields(message:CaptureHTTPMessage){return message.cookies.map(cookie=>({...cookie,attributes:cookie.attributes??[]}))}
export function hexPreview(text:string,maxBytes=65536):{lines:string[];limited:boolean}{
 const bytes=new TextEncoder().encode(text.slice(0,maxBytes+1));const shown=bytes.slice(0,maxBytes),lines:string[]=[];
 for(let i=0;i<shown.length;i+=16){const part=shown.slice(i,i+16);lines.push(i.toString(16).padStart(8,"0")+"  "+Array.from(part,v=>v.toString(16).padStart(2,"0")).join(" ").padEnd(47," ")+"  "+Array.from(part,v=>v>=32&&v<127?String.fromCharCode(v):".").join(""))}
 return {lines,limited:bytes.length>maxBytes||text.length>maxBytes};
}
export function coloredTokens(text:string){const limited=text.length>65536;const source=text.slice(0,65536);const parts=source.split(/(<[^<>]{0,4096}>|"(?:[^"\\]|\\.)*"|'[^']*'|\b(?:true|false|null|\d+(?:\.\d+)?)\b)/g).slice(0,10000);return {limited:limited||parts.join("")!==source,tokens:parts.map(text=>({text,kind:text.startsWith("<")?"tag":/^["']/.test(text)?"string":/^(true|false|null|\d)/.test(text)?"value":"text"}))}}
const escape=(text:string)=>text.replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;").replace(/"/g,"&quot;");
// Preserve only attribute-free formatting tags. All URLs, attributes, scripts,
// styles, forms, embedded documents and navigation markup become literal text.
export function inertHtml(text:string):string{
 const parts=text.slice(0,65536).split(/(<\/?(?:p|div|span|b|strong|i|em|u|br|hr|h[1-6]|ul|ol|li|table|thead|tbody|tr|th|td|pre|code|blockquote)\s*\/?>)/gi);
 const safe=parts.map((part,i)=>i%2?part:escape(part)).join("");return '<!doctype html><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src \'none\'; base-uri \'none\'; form-action \'none\'">'+safe;
}
export function safeRaster(message:CaptureHTTPMessage):string|null{const image=message.imagePreview;if(!image||image.mime!=="image/png"||image.width<1||image.height<1||image.width>2048||image.height>2048||image.width*image.height>1048576||image.base64.length>1398104||!/^iVBORw0KGgo[A-Za-z0-9+/=]*$/.test(image.base64))return null;return `data:image/png;base64,${image.base64}`}
export function streamText(row:HttpCaptureTransactionRow,mode:"sent"|"received"|"both"):string {const sent=buildRawRequest(row).text,received=buildRawResponse(row).text;return mode==="sent"?sent:mode==="received"?received:sent+"\r\n\r\n"+received}
