#!/usr/bin/env bash
# ${greeting^^} needs bash 4. The compleo_* variables themselves are plain assignments.
#
# Minimal example of a bash script using compleo for its own CLI parsing.
#   mise run build && PATH="$PWD/bin:$PATH" examples/greet.bash world --times 2 --loud

eval "$(compleo up <<'SPEC'
description: Print a greeting.
flags:
  --loud: {description: shout the greeting}
  --times=: {description: how many times to repeat, default: "1", type: {kind: int, min: 1}}
positionals:
  - name: name
    description: who to greet
    required: true
SPEC
)"

greeting="Hello, ${compleo_name}!"
[[ ${compleo_loud} == true ]] && greeting=${greeting^^}

for ((i = 0; i < compleo_times; i++)); do
  printf '%s\n' "$greeting"
done
