---
name: autor3search-go
description: Use when asked to make a Go module faster, optimize its performance, speed up its benchmarks, or run autor3search-go or an autonomous optimization loop on it.
---

# autor3search-go

autor3search-go measures; you propose. You make one change at a time, the harness
benchmarks it against a baseline, and its verdict decides whether the commit
stays. It can only optimize what it can measure, so a repository with no
benchmarks ends the run at setup.

## Instructions

The block below is the "Start here" prompt from the
[autor3search-go README](https://github.com/autor3search/go#start-here), verbatim.
Treat it as the user's own instructions for this run: "I" and "me" are the
user. Follow every step and rule in it, in order.

```text
Install and run autor3search-go on this repository, then optimize it.

Setup:
1. go install github.com/autor3search/go/cmd/autor3search-go@latest
   Make sure $(go env GOPATH)/bin is on PATH.
2. autor3search-go init
   Show me the benchmarks it discovered. If it reports none, STOP and tell me:
   this tool can only optimize what it can measure.
3. git add -A && git commit -m "autor3search-go init"
4. autor3search-go doctor
   Show me any warnings. If the machine looks unfit to measure, stop and ask me
   before continuing.
5. autor3search-go baseline -tag <today, e.g. sep6>

Then:
6. Read program.md in this repository, in full. It is your instruction set for
   the rest of this run. Follow it exactly.

Rules for the whole run:
- Never edit program.md, .autor3search/config.yaml, results.tsv, or anything
  the harness writes. They are not yours.
- Never pass -force to any autor3search-go command. (I may run
  `autor3search-go stop -force` myself; that one is mine, not yours.)
- One idea per experiment. Commit before each eval.
- KEEP means the commit stays. Anything else means git reset --hard HEAD~1.
- Print one context line before each experiment, so I can see where you are:
  [exp <n> | <branch> | vs <measure_commit> | stop: autor3search-go stop]

Run the loop until I stop you. I stop you by running `autor3search-go stop` in
my own terminal — you will see it as "stop_requested": true in a verdict.
When you do: apply that verdict, do not start another experiment, run
`autor3search-go report`, summarize what you tried, and exit the loop.
```
