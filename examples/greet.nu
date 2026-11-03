#!/usr/bin/env nu
# compleo parse --shell=nu prints a nuon record. Nu resolves `source` before the
# script runs, so this reads that record instead of sourcing generated code.
#
#   mise run build && PATH="$PWD/bin:$PATH" examples/greet.nu world --times 2 --loud

def --wrapped main [...args: string] {
  let name = ($env.CURRENT_FILE | path basename)
  let spec = r#'
description: Print a greeting.
flags:
  --loud: {description: shout the greeting}
  --times=: {description: how many times to repeat, default: "1", type: {kind: int, min: 1}}
positionals:
  - name: name
    description: who to greet
    required: true
'#
  let raw = ($spec | ^compleo parse --shell=nu --name $name -- ...$args | complete)
  let result = ($raw.stdout | from nuon)
  match $result.kind {
    "error" => {
      print --stderr $result.message
      exit 2
    }
    "help" => {
      print $result.text
      exit 0
    }
    "generated" => {
      print --stderr $result.message
      exit 0
    }
    _ => {}
  }

  let greeting = (if $result.compleo_loud == "true" {
    $"Hello, ($result.compleo_name)!" | str uppercase
  } else {
    $"Hello, ($result.compleo_name)!"
  })
  for _ in 0..<($result.compleo_times | into int) {
    print $greeting
  }
}
