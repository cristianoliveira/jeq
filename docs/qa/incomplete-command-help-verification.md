# Incomplete command help verification

Task: TASK-0034

Bare action commands use native Cobra help before dependency work:

| Surface | Bare behavior | Help channel/status |
|---|---|---|
| `jeq` | root Cobra help | stdout / 0 |
| `jeq examples` | parent Cobra help | stdout / 0 |
| `jeq examples <recipe>` | recipe Cobra help | stdout / 0 |
| `jeq ask` | action Cobra help | stdout / 0 |
| `jeq validate` | action Cobra help | stdout / 0 |
| `jeq map` | action Cobra help | stdout / 0 |
| `jeq reduce` | action Cobra help | stdout / 0 |
| `jeq gate` | action Cobra help | stdout / 0 |
| `jeq version` | version result | stdout / 0 |
| `jeq models` | model result or operational error | stdout or stderr |
| `jeq completion bash` | Cobra completion script | stdout / 0 |

Each bare action is byte-identical to its explicit `--help` form. Action-specific
flags switch the command into real invocation and preserve validation and exit
classification. Parameterized tests verify no environment, file, stdin, client,
or network dependency is touched by bare actions.
