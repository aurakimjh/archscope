from pathlib import Path
source=Path('apps/engine-native/internal/parsers/jenniferprofile/testdata/sample_count_mismatch.txt').read_text(encoding='utf-8')
source=source.replace('Total Transaction : 4','Total Transaction : 1').replace('922846047875413046','1002').replace('2026050908316771PCMONLxtz8x0000001','synthetic-incomplete-guid')
source='\n'.join(line for line in source.splitlines() if not line.endswith(' END') and 'TOTAL[' not in line)+'\n'
Path('.tmp/windows-final-20261114/ui-fixtures/msa-incomplete.txt').write_text(source,encoding='utf-8')
print('Declared1 actual1 profile with START and deliberately missing END')
