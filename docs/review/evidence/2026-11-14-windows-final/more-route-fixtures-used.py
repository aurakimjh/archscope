from pathlib import Path
out=Path('.tmp/windows-final-20261114/ui-fixtures')
events=[]
for n in range(35):
    frames=[f'2026-11-15T00:20:{n:02d}+09:00 java.lang.IllegalStateException: acceptance case {n}', f'    at app.Case{n}.run(Case{n}.java:10)']
    frames += [f'    at app.deep.ProcessingStage{j}.handleSyntheticBoundaryRequest(ProcessingStage{j}.java:{100+j})' for j in range(50)]
    events.append('\n'.join(frames))
(out/'exception-35-long.txt').write_text('\n\n'.join(events)+'\n',encoding='utf-8')
print('35 synthetic exceptions with51frames each')
