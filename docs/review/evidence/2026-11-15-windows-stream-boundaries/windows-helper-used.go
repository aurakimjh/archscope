// Run from a private directory inside the engine module (Go internal visibility).
package main

import (
 "encoding/json"
 "fmt"
 "net/http"
 "os"
 "strings"
 "time"

 "github.com/aurakimjh/archscope/apps/engine-native/internal/capture/requestbuilder"
 "github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
 "github.com/aurakimjh/archscope/apps/engine-native/internal/capture/summary"
 "github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func check(ok bool, message string) { if !ok { panic(message) } }
func main() {
 base, destination, receipt := os.Args[1], os.Args[2], os.Args[3]
 client := requestbuilder.NewClient()
 run := fmt.Sprintf("%x",time.Now().UnixNano())
 rows := []models.CaptureTransaction{}
 add := func(tx models.CaptureTransaction) {
  tx.Sequence = len(rows)+1
  rows = append(rows,tx)
  fmt.Printf("%s state=%s status=%d storage=%s wire-body=%d decoded=%d preview=%d error=%s\n",tx.Path,tx.State,tx.StatusCode,tx.Response.BodyStorage,tx.Response.BodySize,tx.Response.BodyDecoded,len(tx.Response.BodyPreview),tx.Error)
 }
 send := func(id,path string,timeout int,method string) models.CaptureTransaction {
  tx,err:=client.Send(requestbuilder.Request{ID:id,URL:base+path,Method:method,TimeoutSeconds:timeout})
  if err!=nil {panic(err)}
  return tx
 }
 for _,path:=range []string{"identity","identity-short","x-gzip","stacked","binary","binary-gzip","binary-exact","stream"} {
  tx:=send(path,"/"+path,5,"GET")
  check(tx.StatusCode==200,path+" status")
  switch path {
  case "identity","x-gzip":
   check(tx.Response.BodyStorage=="inline"&&tx.Response.BodyPreview==`{"marker":"identity-proof"}`&&tx.Response.BodyDecoded==27,path+" decode")
  case "identity-short":
   check(tx.State==models.TxFailed&&tx.Response.BodySize==5&&tx.Response.BodyPreview=="short"&&strings.Contains(tx.Error,"unexpected EOF"),"identity short")
  case "stacked":
   check(tx.Response.BodyStorage=="encoded"&&tx.Response.BodyDecoded==-1&&tx.Response.BodyEncoding=="gzip, br"&&tx.Response.BodyPreview=="", "stacked must stay unavailable")
  case "binary","binary-gzip":
   check(tx.Response.BodyStorage=="truncated"&&tx.Response.BodyDecoded==1048576&&tx.Response.BodyPreview==""&&tx.Response.ImagePreview==nil,path+" cap")
  case "binary-exact":
   check(tx.Response.BodyStorage=="binary"&&tx.Response.BodyDecoded==1048576&&tx.Response.BodySize==1048576,path+" exact cap")
  case "stream":
   check(tx.State==models.TxComplete&&tx.Response.BodyPreview=="first\nsecond\nthird\n"&&tx.Response.BodySize==19,"finite chunked body")
  }
  add(tx)
 }
 waitPrefix:=func(id string){
  deadline:=time.Now().Add(3*time.Second)
  for time.Now().Before(deadline) {
   resp,err:=http.Get(base+"/stats");if err!=nil {panic(err)}
   var events []struct {Path,Stage string};err=json.NewDecoder(resp.Body).Decode(&events);resp.Body.Close();if err!=nil {panic(err)}
   for _,event:=range events {if strings.Contains(event.Path,"id="+id)&&event.Stage=="prefix" {return}}
   time.Sleep(20*time.Millisecond)
  }
  panic("server prefix not emitted: "+id)
 }
 cancellation:=map[string]bool{}
 for _,path:=range []string{"hold","hold-gzip"} {
  id:="cancel-"+run+"-"+path
  done:=make(chan models.CaptureTransaction,1)
  go func(){done<-send(id,"/"+path+"?id="+id,5,"GET")}()
  waitPrefix(id)
  _,duplicateErr:=client.Send(requestbuilder.Request{ID:id,URL:base+"/identity?duplicate="+id})
  check(duplicateErr!=nil&&strings.Contains(duplicateErr.Error(),"already sending"),"duplicate in-flight ID")
  check(!client.Cancel("unrelated-id"),"wrong cancellation ID")
  // Prefix emission is independently observed before this bounded read window.
  time.Sleep(250*time.Millisecond)
  check(client.Cancel(id),"active cancellation")
  var tx models.CaptureTransaction
  select {case tx=<-done:case <-time.After(2*time.Second):panic("cancel did not finish")}
  check(tx.StatusCode==200&&tx.State==models.TxFailed&&strings.Contains(tx.Error,"context canceled"),"cancel after headers")
  if path=="hold" {
   check(tx.Response.BodyPreview=="prefix\n"&&tx.Response.BodySize==7&&tx.Response.BodyDecoded==7,"plain prefix accounting")
  } else {
   check(tx.Response.BodyStorage=="encoded"&&tx.Response.BodyDecoded==-1&&tx.Response.BodyPreview==""&&tx.Response.BodyEncoding=="gzip","canceled gzip prefix must not masquerade as decoded body")
  }
  check(!client.Cancel(id),"completed request must release ID")
  cancellation[path]=true
  add(tx)
  if path=="hold" {
   tx=send(id,"/identity?after-cancel=1",5,"GET")
   check(tx.State==models.TxComplete&&tx.Response.BodyPreview==`{"marker":"identity-proof"}`,"reuse after cancellation")
   add(tx)
  }
 }
 timeout:=send("deadline","/timeout?id=deadline",1,"GET")
 check(timeout.StatusCode==200&&timeout.State==models.TxFailed&&strings.Contains(timeout.Error,"deadline exceeded")&&strings.HasPrefix(timeout.Response.BodyPreview,"prefix\n")&&timeout.Response.BodySize==int64(len(timeout.Response.BodyPreview)),"deadline after headers")
 check(!client.Cancel("deadline"),"timeout releases ID")
 add(timeout)
 head:=send("head","/head",5,"HEAD")
 check(head.State==models.TxComplete&&head.Response.BodySize==0&&head.Response.BodyDecoded==0,"HEAD has no response body")
 add(head)
 document:=sessionfile.Document{Type:sessionfile.Kind,Version:1,Scope:"all",SelectedID:rows[0].ID,Transactions:rows}
 data,err:=json.MarshalIndent(document,"","  ");if err!=nil {panic(err)}
 if err=os.WriteFile(destination,data,0600);err!=nil {panic(err)}
 result:=struct {Passed bool `json:"passed"`;Count int `json:"count"`;Cancellation map[string]bool `json:"cancellation"`;Rows []models.CaptureTransaction `json:"rows"`;Summary summary.Snapshot `json:"summary"`}{true,len(rows),cancellation,rows,summary.Build(rows)}
 data,err=json.MarshalIndent(result,"","  ");if err!=nil {panic(err)}
 if err=os.WriteFile(receipt,data,0600);err!=nil {panic(err)}
}
