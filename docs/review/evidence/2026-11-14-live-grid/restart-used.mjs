import {spawn} from 'node:child_process';
const base='C:/workspace/archscope/.tmp/live-grid-20261114',arch=process.argv[2],root=base+'/'+arch+'-fixed';
const child=spawn(base+'/archscope-live-'+arch+'.exe',['--automation-descriptor',root+'/.private-restart.json','--open',root+'/live-after.httpsession'],{stdio:'ignore',windowsHide:false});
await new Promise(resolve=>child.once('exit',resolve));
