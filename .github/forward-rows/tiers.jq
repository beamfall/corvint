# SPDX-License-Identifier: Apache-2.0
# AFP-V0-480: the shadow comparison tiers of one change, from static `go list` metadata.
# Usage: jq -n -c --rawfile changed FILE --slurpfile pkgs GOLIST --arg module PATH -f tiers.jq
# FILE is `git diff --name-only --no-renames BASE TARGET` (one path per line), GOLIST the
# `go list -e -json=ImportPath,Imports,TestImports,XTestImports,EmbedFiles,TestEmbedFiles,
# XTestEmbedFiles,EmbedPatterns,TestEmbedPatterns,XTestEmbedPatterns ./...` stream of the module
# PATH. Prints {tiers, tierIncomplete}:
#   changed (B): the packages whose directory holds a changed .go file;
#   hop1 (C):    B plus every package that imports or test-imports one of them;
#   hop2 (D):    C plus every package that imports or test-imports one of those.
# A change no package graph can bound makes every tier the string "FULL", with the reason in
# tiers.full, never an empty list: a .github/ path, go.mod/go.sum/go.work, a testdata path, a
# file a package embeds, any other non-Go file, or a .go file outside every listed package.
# --rawfile deleted (optional) lists the deleted paths: the target's go list cannot name a deleted
# file as embedded, so one in or under a package directory with any embed pattern is FULL too.
# AFP-V0-520: two kinds of non-Go path are exempt from the non-Go rule only, never from an earlier
# one: a sealed CEM directly under .corvint/changes/ (the AFP-V0-152 rule), and a path that
# --rawfile documentation (optional; the bytes of .corvint/documentation-paths.json) declares as
# documentation under AFP-V0-160. A declaration AFP-V0-160 (1) would refuse declares nothing.
def rel($m): if . == $m then "" elif startswith($m + "/") then .[($m | length) + 1:] else null end;
def parent: if test("/") then sub("/[^/]*$"; "") else "" end;
def sealedCEM: startswith(".corvint/changes/") and (.[17:] | endswith(".cem.json") and length > 9 and (contains("/") | not));
def validEntry: type == "string" and (rtrimstr("/") as $t
  | ($t | startswith("/") | not) and ($t | contains("\\") | not) and ($t | explode | all(. != 0))
  and ($t | split("/") | all(. != "" and . != "." and . != ".."))
  and ([".git", ".corvint"] | all(. as $r | $t != $r and ($t | startswith($r + "/") | not))));
def documentation($raw): (try ($raw | if utf8bytelength <= 65536 then fromjson else null end) catch null)
  | if type == "object" and (keys == ["paths", "profile"]) and .profile == "corvint-documentation-paths/0"
       and (.paths | type == "array" and length <= 256 and all(validEntry)
            and ([range(1; length) as $i | .[$i - 1] < .[$i]] | all))
    then .paths else [] end;
[$changed | split("\n")[] | select(length > 0)] as $files
| [$pkgs[] | select(type == "object" and (.ImportPath | type) == "string")
   | . as $p | (.ImportPath | rel($module)) as $dir | select($dir != null)
   | {path: .ImportPath, dir: $dir,
      imports: ([(.Imports // []), (.TestImports // []), (.XTestImports // [])] | add | unique - [$p.ImportPath]),
      embeds: ([(.EmbedFiles // []), (.TestEmbedFiles // []), (.XTestEmbedFiles // [])] | add
               | map(if $dir == "" then . else $dir + "/" + . end)),
      embedPatterns: ([(.EmbedPatterns // []), (.TestEmbedPatterns // []), (.XTestEmbedPatterns // [])] | add | length > 0)}] as $list
| ([($ARGS.named.deleted // "") | split("\n")[] | select(length > 0)] | map({key: ., value: true}) | from_entries) as $deleted
| documentation($ARGS.named.documentation // "") as $documentation
| def declared: . as $f | any($documentation[]; . as $e | $f == $e or (($e | endswith("/")) and ($f | startswith($e))));
  ([$list[] | select(.embedPatterns) | .dir] | map({key: ., value: true}) | from_entries) as $embedDirs
| def underEmbed: parent as $d
    | any(($d | split("/")) as $p | range(0; ($p | length) + 1) | $p[0:.] | join("/"); $embedDirs[.]);
  ($list | map({key: .dir, value: .path}) | from_entries) as $byDir
| ([$list[].embeds[]] | map({key: ., value: true}) | from_entries) as $embedded
| [$files[] as $f
   | if ($f | startswith(".github/")) then "the change touches .github/: \($f)"
     elif ($f | split("/") | last | IN("go.mod", "go.sum", "go.work", "go.work.sum")) then "a module file changed: \($f)"
     elif ($f | split("/") | any(. == "testdata")) then "a testdata file changed: \($f)"
     elif $embedded[$f] then "a file a package embeds changed: \($f)"
     elif $deleted[$f] and ($f | underEmbed) then "a deleted file may have been embedded: \($f)"
     elif ($f | endswith(".go") | not) then
       if ($f | sealedCEM) or ($f | declared) then empty else "a non-Go file changed: \($f)" end
     elif $byDir[$f | parent] == null then "a Go file outside every listed package changed: \($f)"
     else empty end] as $full
| ([$files[] | select(endswith(".go")) | $byDir[parent] | select(. != null)] | unique) as $B
| def importers($set): ($set | map({key: ., value: true}) | from_entries) as $s
    | ($set + [$list[] | select(any(.imports[]; $s[.])) | .path]) | unique;
  importers($B) as $C
| importers($C) as $D
| (if ($full | length) > 0 then "FULL" else null end) as $all
| {tiers: {changed: ($all // $B), hop1: ($all // $C), hop2: ($all // $D),
           full: ($full | unique | map(if length > 300 then .[0:300] + "..." else . end))[0:20],
           universe: ($list | length)},
   tierIncomplete: (if ($list | length) == 0 then ["go list named no package of module \($module)"] else [] end)}
