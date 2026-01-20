This file is NOT a unit test.

The project intentionally avoids unit tests per the project requirements.

However, the `/proc/net/nf_conntrack` format is tricky and depends on kernel/module settings.
During development you can:

- Save a real conntrack dump into `.code/example.txt`.
- Run the exporter with `--path.procfs` pointing at a directory that contains `net/nf_conntrack`.

This document exists to capture constraints and assumptions of the parser:

1. We primarily rely on repeated key=value tokens: `src=`, `dst=`, `sport=`, `dport=`, `packets=`, `bytes=`.
2. We use the FIRST occurrence as "original" direction and the SECOND as "reply".
3. Missing `packets/bytes` is expected when `net.netfilter.nf_conntrack_acct=0`.
4. Protocols like ICMP do not contain ports; `sport/dport` remain empty.

