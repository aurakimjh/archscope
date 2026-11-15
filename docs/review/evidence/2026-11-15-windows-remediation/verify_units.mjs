// Checks real Go profiler outputs through production CSV unit resolution.
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const root = new URL('../../../../', import.meta.url);
const baseline = process.argv.includes('--baseline');
const fixtures = process.argv[2];
assert.ok(fixtures && fixtures !== '--baseline', 'fixture directory required');
const unitNames = ['samples', 'nanoseconds', 'microseconds', 'milliseconds', 'seconds', 'bytes', 'unknown'];
let temporary;
try {
  let helper, resolveUnit;
  if (baseline) {
    // Original helper has no imports; Node 25 can strip its erasable TS types.
    temporary = await mkdtemp(join(tmpdir(), 'archscope-wrev2-baseline-'));
    const oldHelper = execFileSync('git', ['show', 'fbf2ce2c:apps/engine-native/cmd/archscope-app/frontend/src/state/chartStudioExport.ts'], {cwd:fileURLToPath(root)});
    const oldPage = execFileSync('git', ['show', 'fbf2ce2c:apps/engine-native/cmd/archscope-app/frontend/src/pages/ChartStudioPage.tsx'], {cwd:fileURLToPath(root), encoding:'utf8'});
    const unit = oldPage.match(/id: "profiler\.components",\s*unit: "([^"]+)"/)?.[1];
    assert.ok(unit);
    const module = join(temporary, 'old-chart.ts');
    await writeFile(module, oldHelper);
    helper = await import(pathToFileURL(module));
    resolveUnit = () => unit;
  } else {
    helper = await import(new URL('apps/engine-native/cmd/archscope-app/frontend/.tmp/state-tests/state/chartStudioExport.js', root));
    resolveUnit = summary => helper.chartValueUnit('profiler.components', 'unknown', summary);
  }
  const cases = [];
  for (const expectedUnit of unitNames) {
    const result = JSON.parse(await readFile(join(fixtures, expectedUnit + '.json'), 'utf8'));
    assert.equal(result.summary.value_unit, expectedUnit);
    assert.equal(result.type, 'profiler_collapsed');
    const rows = result.series.component_breakdown.map(row => ({label:row.component, value:row.samples}));
    assert.equal(rows[0].value, 1000000);
    const snapshot = JSON.stringify(result);
    const unit = resolveUnit(result.summary);
    const csv = helper.chartRowsToCSV(rows, {
      template:'profiler.components', unit, resultType:result.type, sourceFiles:result.source_files,
    });
    assert.equal(JSON.stringify(result), snapshot);
    assert.ok(csv.includes('Other,1000000,' + unit + ',profiler.components,profiler_collapsed,'));
    const sourceCell = '"' + JSON.stringify(result.source_files).replace(/"/g, '""') + '"';
    assert.ok(csv.includes(sourceCell));
    if (expectedUnit === 'nanoseconds') assert.equal(result.summary.estimated_seconds, 0.001);
    cases.push({expectedUnit, exportedUnit:unit, value:1000000, sourceCount:result.source_files.length, passed:unit===expectedUnit, csvSha256:createHash('sha256').update(csv).digest('hex')});
  }
  const passed = cases.every(c => c.passed);
  console.log(JSON.stringify({mode:baseline?'pre-fix fbf2ce2c':'fixed production helper',passed,cases},null,2));
  if (!passed) process.exitCode = 1;
} finally {
  if (temporary) await rm(temporary, {recursive:true,force:true});
}
