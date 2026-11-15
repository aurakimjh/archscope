// Source-bound reproduction; no browser, network, or original evidence mutation.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { chartRowsToCSV } from '../../../../apps/engine-native/cmd/archscope-app/frontend/src/state/chartStudioExport.ts';

const root = new URL('../../../../', import.meta.url);
const page = readFileSync(new URL('apps/engine-native/cmd/archscope-app/frontend/src/pages/ChartStudioPage.tsx', root), 'utf8');
const declaredUnit = page.match(/id: "profiler\.components",\s*unit: "([^"]+)"/)?.[1];
assert.ok(declaredUnit, 'template declaration not found');
const result = JSON.parse(readFileSync(process.argv[2], 'utf8'));
assert.equal(result.type, 'profiler_collapsed');
assert.equal(result.summary.value_unit, 'nanoseconds');
assert.equal(result.summary.estimated_seconds, 0.001);
const rows = result.series.component_breakdown.map(row => ({ label: row.component, value: row.samples }));
const csv = chartRowsToCSV(rows, {
  template: 'profiler.components', unit: declaredUnit,
  resultType: result.type, sourceFiles: ['synthetic-unit-reproduction'],
});
assert.equal(rows[0].value, 1000000);
assert.equal(declaredUnit, 'samples');
assert.notEqual(declaredUnit, result.summary.value_unit);
assert.ok(csv.includes(',1000000,samples,profiler.components,'));
const prefix = new URL('docs/review/evidence/2026-11-14-windows-final/', root);
const remaining = JSON.parse(readFileSync(new URL('remaining-acceptance.json', prefix), 'utf8'));
const text = readFileSync(new URL('jfr-notable-dense-fixed.txt', prefix), 'utf8');
const counts = JSON.parse(readFileSync(new URL('jfr-binary-counts.json', prefix), 'utf8'));
assert.equal(counts.totalEvents, 3010);
assert.match(text, /필터된 이벤트\r?\n도움말 보기\r?\n3,010/);
assert.match(text, /스택 샘플\r?\n도움말 보기\r?\n153/);
const inventory = remaining['T-627'].routes.find(route => route.route === 'jfr').verifiedThisFollowup;
assert.ok(inventory.includes('153filtered events'));
console.log(JSON.stringify({
  scope: 'actual Go result and production CSV helper; source-selected template; no native UI rerun',
  sourcePath: fileURLToPath(new URL('apps/engine-native/cmd/archscope-app/frontend/src/pages/ChartStudioPage.tsx', root)).split('/').slice(-4).join('/'),
  chartUnits: { reproduced: true, originalUnit: result.summary.value_unit, exportedUnit: declaredUnit, value: rows[0].value, durationSeconds: result.summary.estimated_seconds, sourceLabelsSanitized: true, csv },
  jfrInventory: { reproduced: true, originalFilteredEvents: 3010, stackSamples: 153, mislabeledFilteredEvents: 153 },
}, null, 2));
