package main
import (
 "encoding/json"
 "fmt"
 "os"
 "github.com/aurakimjh/archscope/apps/engine-native/internal/capture/requestbuilder"
 "github.com/aurakimjh/archscope/apps/engine-native/internal/models"
 "github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
)
func main(){
 rows:=[]models.CaptureTransaction{}
 for _, name:=range []string{"gzip","encoded","short","large","error","delay"}{
  tx,err:=requestbuilder.NewClient().Send(requestbuilder.Request{ID:"matrix-"+name,Method:"GET",URL:"http://127.0.0.1:11515/"+name,Headers:[]models.HeaderField{{Name:"Accept-Encoding",Value:"gzip"}}})
  if err!=nil{panic(err)}
  rows=append(rows,tx)
  fmt.Printf("%s state=%s status=%d storage=%s body=%d decoded=%d preview=%d error=%s\n",name,tx.State,tx.StatusCode,tx.Response.BodyStorage,tx.Response.BodySize,tx.Response.BodyDecoded,len(tx.Response.BodyPreview),tx.Error)
 }
 doc:=sessionfile.Document{Type:sessionfile.Kind,Version:1,Scope:"all",SelectedID:rows[0].ID,Transactions:rows}
 data,err:=json.MarshalIndent(doc,"","  ");if err!=nil{panic(err)}
 if err=os.WriteFile(os.Args[1],data,0600);err!=nil{panic(err)}
}
