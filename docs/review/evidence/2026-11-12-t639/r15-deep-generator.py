"""Reproduce only the synthetic raw text used by r15-deep-cli.json."""
from pathlib import Path
import sys

directory = Path(sys.argv[1])
directory.mkdir(parents=True, exist_ok=True)
for count in (20000, 24000):
    with (directory / f"{count}.perf").open("w", newline="\n") as output:
        stack = "".join(
            f"    {0x400000+i:x} synthetic_deep_function_{i:02d} (/usr/lib/synthetic-module.so)\n"
            for i in range(25)
        )
        for i in range(count):
            output.write(
                f"synthetic   123/124   [{i%8:03d}] {i//1000}.{(i%1000)*1000000+1:09d}: 1000000 cpu-clock:HG:\n"
                + stack
                + "\n"
            )
