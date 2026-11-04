#!/usr/bin/env bash
# ssh-heredoc-check.sh: refuse an ssh command whose remote script is a heredoc inside a quoted
# argument (ASD-V0-769, V1-3868). In `ssh HOST '... <<EOF ... EOF'` the heredoc body sits inside a
# single-quoted word, so one apostrophe in the body ends the quote and the rest runs as local shell.
# The safe form sends the script on stdin: `ssh HOST 'bash -s' <<'EOF'`.
#
#   script/ssh-heredoc-check.sh 'COMMAND'        or   hook JSON on stdin (.tool_input.command)
#
# Quote-aware scan: inside an ssh command, a `<<` inside single or double quotes is refused (exit 2,
# naming the safe form). An unquoted `<<` (stdin heredoc to ssh) is allowed: its body, from the next
# line to the terminator line, is skipped and the scan resumes with the command after it, so a
# later command on the same input is still checked. An unquoted ; | & or newline ends the ssh
# command, and so do ( ) and a backquote (subshells, command substitutions, case arms); `>&`, `<&` and `&>` are redirections.
# An unquoted `#` that starts a word begins a comment, skipped to the end of its line, so a quote or
# `<<` in it opens nothing and its newline still ends the command; a backslash-newline joins lines.
# `$'...'` honours backslash escapes. Leading `NAME=value` assignments, the prefix words env, exec,
# command, nohup and time, and the reserved words ! { } if then else elif do while until do not take
# the command-name position, so `X=1 ssh HOST '... <<EOF'` is still an ssh command; the name is
# compared dequoted, so `\ssh` and `'ssh'` count. The SCRIPT of `sh|bash|dash|ksh|zsh [-opts] -c
# SCRIPT` is scanned as a command line of its own. Exit 0 when allowed.
set -uo pipefail
export LC_ALL=C
shopt -s extglob
if (($#)); then cmd=$1; else
  command -v jq > /dev/null 2>&1 || { printf 'ssh-heredoc-check: jq is not installed\n' >&2; exit 2; }
  cmd=$(jq -r '.tool_input.command // empty') || exit 2
fi
q='' word='' val='' inssh=0 cmdstart=1 shc=0 dl=0 pdl=0 i=0 n=${#cmd} c=''
delims=() # heredoc terminators pending for the body that starts at the next newline
# word marks that a word has started (quotes count); val is its dequoted text.
flush() {
  if [[ -n $word ]]; then
    if ((cmdstart)); then
      case $val in
        [A-Za-z_]*=* | env | exec | command | nohup | time | '!' | '{' | '}' | if | then | else | elif | do | while | until) ;; # the command name is still to come
        *)
          [[ ${val##*/} == ssh ]] && inssh=1
          case ${val##*/} in sh | bash | dash | ksh | zsh) shc=1 ;; esac
          cmdstart=0 ;;
      esac
    elif ((shc == 1)); then
      case $val in --*) ;; -*c*) shc=2 ;; -*) ;; *) shc=0 ;; esac # SHELL [-opts] -c SCRIPT
    elif ((shc == 2)); then
      shc=0
      "${BASH_SOURCE[0]}" "$val" || exit 2 # the -c script is a command line of its own
    fi
    word='' val=''
  fi
}
endcmd() { flush; inssh=0; cmdstart=1; shc=0; }
skip_bodies() { # i is at a newline: consume one body per pending terminator, in order
  local d line strip
  for d in ${delims[@]+"${delims[@]}"}; do
    strip=0; [[ $d == -* ]] && { strip=1; d=${d#-}; }
    while ((i < n)); do
      i=$((i + 1)) # past the newline
      line=${cmd:i}; line=${line%%$'\n'*}
      i=$((i + ${#line}))
      ((strip)) && line=${line##*([[:space:]])}
      [[ $line == "$d" ]] && break
    done
  done
  delims=()
}
for ((i = 0; i < n; i++)); do
  c=${cmd:i:1} pdl=$dl dl=0
  if [[ $q == "'" ]]; then
    [[ $c == "'" ]] && { q=''; continue; }
    val+=$c
  elif [[ $q == '$' ]]; then # $'...': a backslash escapes the next character
    [[ $c == '\' ]] && { i=$((i + 1)); val+=${cmd:i:1}; continue; }
    [[ $c == "'" ]] && { q=''; continue; }
    val+=$c
  elif [[ $q == '"' ]]; then
    [[ $c == '\' ]] && { i=$((i + 1)); val+=${cmd:i:1}; continue; }
    [[ $c == '"' ]] && { q=''; continue; }
    val+=$c
  else
    case $c in
      "'" | '"')
        q=$c
        if ((pdl)) && [[ $c == "'" ]]; then q='$'; val=${val%\$}; fi
        word+=x; continue ;;
      '\')
        # A backslash-newline is a line continuation: it joins the lines and adds nothing.
        [[ ${cmd:i+1:1} == $'\n' ]] && { i=$((i + 1)); continue; }
        i=$((i + 1)); word+=x; val+=${cmd:i:1}; continue ;;
      ' ' | $'\t') flush; continue ;;
      $'\n') endcmd; ((${#delims[@]})) && skip_bodies; continue ;;
      '&')
        # >&, <& and &> are redirections, not command separators.
        if ((i > 0)) && [[ ${cmd:i-1:1} == [\<\>] ]] || [[ ${cmd:i+1:1} == '>' ]]; then word+=$c; val+=$c; continue; fi
        endcmd; continue ;;
      ';' | '|' | '(' | ')' | '`') endcmd; continue ;; # subshells, $( ), backquotes and case arms
      '#') # a comment only at a word start; elsewhere # is part of the word
        if [[ -z $word ]]; then cm=${cmd:i}; cm=${cm%%$'\n'*}; i=$((i + ${#cm} - 1)); continue; fi ;;
      '<')
        if [[ ${cmd:i+1:1} == '<' && ${cmd:i+2:1} == '<' ]]; then
          flush; i=$((i + 2)); continue # a here-string has no body; its word is scanned as usual
        elif [[ ${cmd:i+1:1} == '<' ]]; then
          # An unquoted heredoc: read its terminator (quotes and backslashes stripped) and skip its
          # body at the next newline. Inside ssh this is the stdin form, which is the safe one.
          flush; i=$((i + 2)); d=''
          [[ ${cmd:i:1} == '-' ]] && { d='-'; i=$((i + 1)); }
          while [[ ${cmd:i:1} == ' ' || ${cmd:i:1} == $'\t' ]]; do i=$((i + 1)); done
          dq=''
          while ((i < n)); do
            c=${cmd:i:1}
            if [[ -n $dq ]]; then [[ $c == "$dq" ]] && dq='' || d+=$c
            else case $c in
                "'" | '"') dq=$c ;;
                '\') i=$((i + 1)); d+=${cmd:i:1} ;;
                ' ' | $'\t' | $'\n' | ';' | '|' | '&' | '<' | '>') break ;;
                *) d+=$c ;;
              esac
            fi
            i=$((i + 1))
          done
          delims+=("$d"); i=$((i - 1)); continue
        fi
        flush; continue ;;
    esac
    word+=$c val+=$c
    [[ $c == '$' ]] && dl=1
    continue
  fi
  if ((inssh)) && [[ $c == '<' && ${cmd:i+1:1} == '<' ]]; then
    printf "ssh-heredoc-check: REFUSE heredoc inside a quoted ssh argument; an apostrophe in the body ends the quote. Use: ssh HOST 'bash -s' <<'EOF' ... EOF\n" >&2
    exit 2
  fi
  [[ -z $word ]] && word=x
done
flush # a trailing -c script is checked too
exit 0
