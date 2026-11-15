// Execute from a private folder inside the engine module for internal visibility.
package main

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"

 analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/httpcapture"
 "github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
)

func main() {
 for _, size := range []int{4096,512,32} {
  file:=filepath.Join(os.Args[1],fmt.Sprintf("synthetic-%d.httpsession",size))
  doc,redaction,err:=sessionfile.Read(file);if err!=nil {panic(err)}
  result:=analyzer.BuildLive(doc.Transactions,file,"portable-session-v1",redaction,analyzer.Options{TransactionRows:100000})
  // Independent canonical projection identity; no source re-read or raw secrets.
  hash:=sha256.New();encoder:=json.NewEncoder(hash)
  rows:=result.Tables["transactions"].([]map[string]any)
  encoder.Encode(len(rows));for _,row:=range rows {if err=encoder.Encode(row);err!=nil {panic(err)}}
  result.Metadata.Extra["workspace_transaction_rows_fingerprint"]="v1:"+hex.EncodeToString(hash.Sum(nil))
  data,err:=json.Marshal(result);if err!=nil {panic(err)}
  if err=os.WriteFile(filepath.Join(os.Args[2],fmt.Sprintf("model-%d.json",size)),data,0600);err!=nil {panic(err)}
 }
}
