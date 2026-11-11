import { runWindowsGo } from './go.mjs';

if (process.platform !== 'win32') throw new Error('Windows tests must run on a native Windows host');
process.exitCode = runWindowsGo('test', process.argv.slice(2));
