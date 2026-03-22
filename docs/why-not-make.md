# Why Not Just Use make?

Short answer: you absolutely can.

`make` is ubiquitous, battle-tested, and already present in many repositories. If your repo is well served by a small Makefile and your team is happy with it, there is no need to replace it just to adopt `fkn`.

`fkn` exists for the cases where a repository needs something a little more explicit than a Makefile alone.

## What make Is Great At

- simple task execution
- environments where `make` is already installed
- lightweight command aliases
- repositories that already have mature Makefile conventions

`fkn` does not win by pretending `make` is bad. It wins when a repo needs a clearer operational interface for both humans and agents.

## Where fkn Is Different

### 1. `fkn` wraps what you already have

`fkn init --from-repo` is designed to sit on top of an existing repo surface.

That means you can keep:

- your `Makefile`
- your `justfile`
- your `package.json` scripts
- your existing Go workflows

and scaffold a first `fkn.yaml` from them.

The adoption pitch is not “throw away your Makefile.”

It is:

> Keep your existing repo commands. Add one structured interface on top.

### 2. YAML is easier for tools and agents to reason about

`make` is familiar, but it is still a language with its own semantics, escaping rules, and historical quirks.

`fkn.yaml` is intentionally boring:

- explicit fields
- predictable structure
- schema validation
- easy JSON-style parsing

That makes it easier for:

- new developers
- editors and language servers
- coding agents
- automation that wants stable structure

### 3. `fkn` models repo intent, not just commands

A Makefile mostly tells you what can be run.

`fkn.yaml` also captures:

- task descriptions
- safety levels
- reusable dependencies via `needs`
- scopes and scope intent
- groups
- prompts
- context configuration
- codemap knowledge
- generated docs for humans and agents

That is the main difference. `fkn` is trying to be the operational API for the repo, not just a command launcher.

### 4. `fkn` is designed for agent workflows

The strongest `fkn` features are not “run a command.”

They are:

- `fkn plan`
- `fkn diff-plan`
- `fkn repair`
- `fkn context`
- `fkn agent-brief`
- `fkn init --docs`

Those features help an agent or a human answer:

- what should I touch?
- what should I run?
- what broke?
- what does this part of the repo do?

That is outside the scope of a normal Makefile.

### 5. `fkn` is installable from source in locked-down environments

This matters more than it sounds.

If Go is already approved in an enterprise environment, then:

```bash
go install github.com/neural-chilli/fkn/cmd/fkn@latest
```

is often acceptable in places where downloading arbitrary binaries or adding extra runtimes is not.

## When make Is Probably Enough

Stick with `make` alone if:

- your repo only needs a handful of tasks
- humans already know the commands
- agents are not a serious workflow requirement
- you do not need richer task metadata or repo docs generation

That is a completely reasonable choice.

## When fkn Is Worth Adding

`fkn` is a better fit when:

- the repo already has scattered commands across multiple tools
- you want one explicit task interface for humans and agents
- you want generated `HUMANS.md`, `AGENTS.md`, and `CLAUDE.md`
- you want planning, repair, and context features around task execution
- you want structured knowledge in one YAML file instead of scattered tribal knowledge

## The Honest Position

`fkn` is not “better than make” in every situation.

It is better for repositories that want:

- one structured operational interface
- stable machine-readable metadata
- a low-friction path from existing repo commands to agent-friendly workflows

If your Makefile already does everything you need, keep it.

If your repo needs a clearer interface for both humans and agents, `fkn` is the layer you add on top.
