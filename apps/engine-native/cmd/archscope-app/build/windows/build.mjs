import { runWindowsGo } from './go.mjs';

process.exitCode = runWindowsGo('build', process.argv.slice(2));
