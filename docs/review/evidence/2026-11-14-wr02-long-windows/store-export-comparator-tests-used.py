"""Synthetic regression tests; these do not execute a Windows desktop."""
from copy import deepcopy
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from urllib.parse import urlsplit

import verify_p2_windows_readback as readback


def transaction(ident='synthetic-2', sequence=2, query='next=/detail?id=7&keep=2'):
    return {'id': ident, 'sequence': sequence, 'url': 'http://127.0.0.1:11510/lookup?' + query,
            'host': '127.0.0.1', 'query': '', 'statusCode': 200, 'state': 'complete',
            'process': {'attribution': 'confirmed', 'key': {'pid': 123, 'startTime': 'synthetic-start'}}}


def portable(tx, starred=False, comment=''):
    result = deepcopy(tx)
    result['host'] = '127.0.0.1:11510'
    result['query'] = tx['url'].split('?', 1)[1]
    result.update(starred=starred, comment=comment)
    return result


class ComparisonTests(unittest.TestCase):
    def setUp(self):
        self.source = transaction()
        self.export = portable(self.source, True, '한글 😀 fixture-note')
        self.annotations = {'synthetic-2': {'starred': True, 'comment': '한글 😀 fixture-note'}}

    def compare(self, exported=None, annotations=None, source=None):
        return readback.compare_transactions([source or self.source], [exported or self.export],
                                            self.annotations if annotations is None else annotations,
                                            'synthetic-2')

    def test_exact_star_and_unicode_note_match_sidecar(self):
        self.assertTrue(self.compare()['annotationsMatch'])

    def test_changed_star_is_rejected(self):
        with self.assertRaises(readback.ReadbackMismatch):
            self.compare({**self.export, 'starred': False})

    def test_changed_note_is_rejected(self):
        with self.assertRaises(readback.ReadbackMismatch):
            self.compare({**self.export, 'comment': 'different'})

    def test_missing_nondefault_annotation_is_rejected(self):
        for field in ['starred', 'comment']:
            with self.subTest(field=field), self.assertRaises(readback.ReadbackMismatch):
                row = dict(self.export)
                del row[field]
                self.compare(row)

    def test_missing_and_explicit_zero_values_are_equivalent(self):
        row = portable(self.source)
        for raw in [{}, {'starred': False}, {'comment': ''}, {'starred': False, 'comment': ''}]:
            with self.subTest(raw=raw):
                actual = {k: v for k, v in row.items() if k not in ('starred', 'comment')}
                actual.update(raw)
                self.assertTrue(self.compare(actual, {})['annotationsMatch'])

    def test_live_inline_annotations_are_replaced_by_absent_sidecar_defaults(self):
        source = {**self.source, 'starred': True, 'comment': 'stale inline'}
        self.assertTrue(self.compare(portable(self.source), {}, source)['annotationsMatch'])
        with self.assertRaises(readback.ReadbackMismatch):
            self.compare(portable(source, True, 'stale inline'), {}, source)

    def test_explicit_clear_overrides_inline_annotation(self):
        source = {**self.source, 'starred': True, 'comment': 'old inline'}
        self.assertTrue(self.compare(portable(self.source),
                        {'synthetic-2': {'starred': False, 'comment': ''}}, source)['annotationsMatch'])

    def test_annotation_types_do_not_coerce_bool_int_null_or_strings(self):
        for values in [{'starred': 1, 'comment': 'x'}, {'starred': 'true'},
                       {'starred': None}, {'comment': None}, {'comment': 4}]:
            with self.subTest(values=values), self.assertRaises(readback.ReadbackMismatch):
                self.compare(annotations={'synthetic-2': values})
            with self.subTest(export=values), self.assertRaises(readback.ReadbackMismatch):
                self.compare({**self.export, **values})

    def test_host_and_query_are_checked_in_export_not_overwritten(self):
        for values in [{'host': 'wrong.test'}, {'query': 'next=/detail'}, {'query': ''}]:
            with self.subTest(values=values), self.assertRaises(readback.ReadbackMismatch):
                self.compare({**self.export, **values})

    def test_nested_query_encoding_and_duplicate_keys_remain_exact(self):
        source = transaction(query='value=??tail?&raw=%3F%26&raw=%3f')
        self.assertTrue(self.compare(portable(source), {}, source)['annotationsMatch'])
        with self.assertRaises(readback.ReadbackMismatch):
            self.compare({**portable(source), 'query': 'value=??tail?&raw=%3f%26&raw=%3f'}, {}, source)

    def test_unrelated_and_future_transaction_fields_are_not_excluded(self):
        source = {**self.source, 'futureField': {'value': 7}}
        actual = {**self.export, 'futureField': {'value': 7}}
        self.assertTrue(self.compare(actual, source=source)['allTransactionFieldsMatchDeclaredPolicy'])
        for row in [self.export, {**actual, 'futureField': {'value': 8}}, {**actual, 'extra': True}]:
            with self.subTest(row=row), self.assertRaises(readback.ReadbackMismatch):
                self.compare(row, source=source)

    def test_identity_set_duplicates_and_missing_selection_are_rejected(self):
        for source, actual, selected in [([self.source] * 2, [self.export], 'synthetic-2'),
                                        ([self.source], [self.export] * 2, 'synthetic-2'),
                                        ([self.source], [{**self.export, 'id': 'other'}], 'other'),
                                        ([self.source], [self.export], 'missing'), ([], [], '')]:
            with self.subTest(selected=selected), self.assertRaises(readback.ReadbackMismatch):
                readback.compare_transactions(source, actual, self.annotations, selected)

    def test_reordering_never_transfers_annotation_to_another_id(self):
        other = transaction('synthetic-3', 3)
        result = readback.compare_transactions([self.source, other], [portable(other), self.export],
                                              self.annotations, 'synthetic-2')
        self.assertEqual(result['rows'], 2)
        with self.assertRaises(readback.ReadbackMismatch):
            readback.compare_transactions([self.source, other],
                    [portable(other, True, '한글 😀 fixture-note'), portable(self.source)],
                    self.annotations, 'synthetic-2')

    def test_valid_orphan_annotation_does_not_attach_to_retained_rows(self):
        notes = {**self.annotations, 'evicted-private-id': {'starred': True, 'comment': 'orphan-note'}}
        result = self.compare(annotations=notes)
        text = json.dumps(result, ensure_ascii=False)
        for private in ['evicted-private-id', 'orphan-note', 'fixture-note', 'synthetic-2']:
            self.assertNotIn(private, text)

    def test_input_objects_are_unchanged(self):
        before = deepcopy((self.source, self.export, self.annotations))
        self.compare()
        self.assertEqual(before, (self.source, self.export, self.annotations))

    def test_policy_states_defaults_and_no_excluded_fields_without_legacy_flag(self):
        report = self.compare()
        self.assertEqual(report['comparisonPolicy']['excludedFields'], [])
        self.assertEqual(report['comparisonPolicy']['missingAnnotationDefaults'],
                         {'starred': False, 'comment': ''})
        self.assertNotIn('allOtherFieldsEqualAfterPortableURLNormalization', report)


class FileAndCLITests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.run_dir = Path(self.temp.name) / 'run'
        self.store = self.run_dir / '.archscope-captures' / 'synthetic-session'
        (self.store / 'rolling').mkdir(parents=True)
        self.report = Path(self.temp.name) / 'report.json'

    def write(self, path, data):
        path.write_text(json.dumps(data, ensure_ascii=False), encoding='utf-8')

    def seed(self, kind='query', annotated=True):
        count = 4 if kind == 'query' else 128
        source = []
        for i in range(count):
            seq = i + (1 if kind == 'query' else 341)
            tx = transaction(f'synthetic-{seq}', seq,
                    'next=/detail?id=7&keep=2' if i == 1 else f'next=/detail?id=3&keep={i}')
            if kind == 'soak':
                tx['url'] = f'http://127.0.0.1:11510/soak?sequence={seq - 1}'
            source.append(tx)
            self.write(self.store / 'rolling' / f'{seq:020}.json', {'transaction': tx})
        selected = source[1] if kind == 'query' else source[0]
        notes = {selected['id']: {'starred': True, 'comment': '한글 😀 fixture-note'}} if annotated else {}
        if annotated:
            self.write(self.store / 'annotations.json', notes)
        rows = [portable(tx, tx['id'] in notes, notes.get(tx['id'], {}).get('comment', '')) for tx in source]
        persisted = 4 if kind == 'query' else 468
        doc = {'transactions': rows, 'selectedId': selected['id'],
               'retention': {'limitRows': 128, 'persisted': persisted, 'retained': count,
                             'evicted': persisted - count}}
        self.write(self.run_dir / '.private-session.json', {'storePath': str(self.store)})
        self.write(self.run_dir / 'final.httpsession', doc)
        stats = {'state': 'finalized', 'captured': persisted, 'persisted': persisted, 'dropped': 0}
        if kind == 'query':
            self.write(self.run_dir / 'seed.json', {'requests': 4, 'retainedURLs': [tx['url'] for tx in source]})
            self.write(self.run_dir / '.private-rows.json', source)
            self.write(self.run_dir / 'command-02-stop.json.done', {'state': 'finalized', 'stats': stats})
        else:
            points = [{'stats': {**stats, 'state': 'running'}, 'checkedRows': 128,
                       'retainedRows': 128, 'hasMainWindow': True, 'Responding': True} for _ in range(6)]
            self.write(self.run_dir / 'result.json', {'schema': 4, 'passed': True,
                'requestedSeconds': 130, 'elapsedSeconds': 130.5, 'requests': 468,
                'pauses': 2, 'clears': 0, 'checkpoints': points, 'finalStats': stats,
                'savedRows': 128, 'savedSessionSha256': hashlib.sha256(
                    (self.run_dir / 'final.httpsession').read_bytes()).hexdigest()})
        return doc

    def cli(self, kind='query', optimized=False):
        return subprocess.run([sys.executable, '-B', *(['-O'] if optimized else []),
                str(Path(readback.__file__).resolve()), '--run-dir', str(self.run_dir),
                '--kind', kind, '--arch', 'amd64', '--report', str(self.report)],
                capture_output=True, text=True)

    def test_query_cli_reads_sidecar_and_publishes_precise_public_report(self):
        self.seed()
        before = {p: p.read_bytes() for p in self.run_dir.rglob('*') if p.is_file()}
        result = self.cli()
        self.assertEqual(result.returncode, 0, result.stderr)
        report = json.loads(self.report.read_text(encoding='utf-8'))
        self.assertTrue(report['annotationsMatch'])
        self.assertEqual(report['schemaVersion'], 2)
        self.assertNotIn('fixture-note', self.report.read_text(encoding='utf-8'))
        self.assertEqual(before, {p: p.read_bytes() for p in before})

    def test_unannotated_missing_sidecar_and_omitempty_fields_pass(self):
        doc = self.seed(annotated=False)
        for tx in doc['transactions']:
            tx.pop('starred')
            tx.pop('comment')
        self.write(self.run_dir / 'final.httpsession', doc)
        self.assertTrue(readback.verify(self.run_dir, 'query', '386')['annotationsMatch'])

    def test_soak_file_integration_preserves_exact_annotation(self):
        self.seed('soak')
        report = readback.verify(self.run_dir, 'soak', '386')
        self.assertEqual(report['rows'], 128)
        self.assertEqual(report['finalState'], 'finalized')
        self.assertTrue(report['annotationsMatch'])

    def test_cli_annotation_corruption_fails_without_publishing_even_under_optimization(self):
        doc = self.seed()
        doc['transactions'][1]['comment'] = 'changed-private-value'
        self.write(self.run_dir / 'final.httpsession', doc)
        for optimized in [False, True]:
            with self.subTest(optimized=optimized):
                result = self.cli(optimized=optimized)
                self.assertEqual(result.returncode, 1)
                self.assertFalse(self.report.exists())
                self.assertNotIn('changed-private-value', result.stdout + result.stderr)

    def test_existing_evidence_report_is_never_overwritten(self):
        self.seed()
        self.report.write_text('original evidence', encoding='utf-8')
        self.assertEqual(self.cli().returncode, 1)
        self.assertEqual(self.report.read_text(encoding='utf-8'), 'original evidence')

    def test_invalid_sidecar_cannot_fall_back_to_no_annotations(self):
        self.seed()
        (self.store / 'annotations.json').write_text('{invalid', encoding='utf-8')
        self.assertEqual(self.cli().returncode, 1)
        self.assertFalse(self.report.exists())

    def test_private_store_must_belong_to_isolated_run(self):
        self.seed()
        self.write(self.run_dir / '.private-session.json', {'storePath': self.temp.name})
        self.assertEqual(self.cli().returncode, 1)
        self.assertFalse(self.report.exists())


if __name__ == '__main__':
    unittest.main()
