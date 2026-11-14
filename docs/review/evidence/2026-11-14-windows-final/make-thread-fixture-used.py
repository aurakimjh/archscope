from pathlib import Path
out=Path('.tmp/windows-final-20261114/ui-fixtures/thread-lock-pair')
out.mkdir(exist_ok=True)
for n in range(2):
    blocks=[f'2026-11-15 00:30:{n*10:02d}', 'Full thread dump Java HotSpot(TM):']
    for i,held,wanted in [(1,'0x00000001','0x00000002'),(2,'0x00000002','0x00000001')]:
        blocks.append(f'''"acceptance-worker-{i}" #{i} prio=5 os_prio=0 tid=0x0000000{i} nid=0x{i} waiting for monitor entry
   java.lang.Thread.State: BLOCKED (on object monitor)
        at app.LockFixture.acquire(LockFixture.java:{10+i})
        - waiting to lock <{wanted}> (a app.FixtureLock)
        - locked <{held}> (a app.FixtureLock)
        at app.LockFixture.run(LockFixture.java:30)
''')
    (out/f'{n+1:02d}-dump.txt').write_text('\n'.join(blocks)+'\n',encoding='utf-8')
print('Two ordered snapshots,2blocked threads/2locks/1cycle each')
