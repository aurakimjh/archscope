from pathlib import Path
import json,copy
out=Path('.tmp/windows-final-20261114/ui-fixtures')
b={'type':'access_log','source_files':['synthetic-before.log'],'created_at':'2026-04-29T10:00:00Z','summary':{'total_requests':100,'avg_response_ms':250,'format_label':'nginx_combined'},'series':{},'tables':{},'charts':{},'metadata':{'parser':'nginx_combined','schema_version':'0.1.0','findings':[]}}
a=copy.deepcopy(b);a['source_files']=['synthetic-after.log'];a['summary']['total_requests']=125;a['summary']['avg_response_ms']=200;a['summary']['new_metric']=3
for name,p in [('report-before.json',b),('report-after.json',a)]: (out/name).write_text(json.dumps(p),encoding='utf-8')
