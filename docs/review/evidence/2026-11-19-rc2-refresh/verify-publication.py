import argparse, json
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('root',type=Path);p.add_argument('--output',type=Path,required=True);a=p.parse_args();r=a.root
load=lambda name:json.loads((r/name).read_text())
release=load('published-release.json');old=load('previous-release.json');latest=load('latest-release.json');tag=load('published-tag.json');obj=load('published-tag-object.json');run=load('publication-run.json');hosted=load('hosted-validation.json');sha='fe92b5bcfd1fdcd3595b30a9d25d2204a8c31937'
assert release['id']==old['id']==447993671
assert release['tag_name']==old['tag_name']=='v0.4.0-rc.2'
assert release['html_url']==old['html_url']
assert release['prerelease'] is True and release['draft'] is False
assert len(release['assets'])==len(old['assets'])==11
assert latest['id']==342706545 and latest['tag_name']=='v0.3.5-24'
assert tag['object']['type']=='tag' and obj['sha']==tag['object']['sha']
assert obj['object']['type']=='commit' and obj['object']['sha']==sha
assert run['headSha']==sha and run['conclusion']=='success' and len(run['jobs'])==4
assert all(j['conclusion']=='success' for j in run['jobs'])
assert hosted['complete'] is True and hosted['candidateCommit']==sha
assert sha in release['body'] and '2026-11-20 (Asia/Seoul)' in release['body']
old_assets={x['name']:x for x in old['assets']}
replaced=[]
for x in release['assets']:
 prev=old_assets[x['name']]
 assert x['id']!=prev['id'],x['name']
 assert x['updated_at']>prev['updated_at'],x['name']
 replaced.append({'name':x['name'],'previousId':prev['id'],'publishedId':x['id'],'contentChanged':x['digest']!=prev['digest']})
receipt={'date':'2026-11-20','status':'PUBLISHED_AND_VERIFIED','tag':release['tag_name'],'sourceCommit':sha,'releaseId':release['id'],'releaseUrl':release['html_url'],'sameReleaseIdAndUrl':True,'prerelease':True,'stableLatestUnchanged':latest['tag_name'],'previousTagObject':load('previous-tag.json')['object']['sha'],'publishedTagObject':tag['object']['sha'],'publishedRun':run['url'],'assetCount':11,'assetReplacements':replaced,'originalAcceptanceLimitsPreserved':True,'limits':'Artifact and hosted regression verification does not establish original-user-file, native GC-hover, clean-host install/upgrade or remaining tracked native acceptance.'}
a.output.write_text(json.dumps(receipt,indent=2)+'\n');print(json.dumps({'status':receipt['status'],'releaseId':receipt['releaseId'],'sourceCommit':sha,'replacedAssets':len(replaced)}))
