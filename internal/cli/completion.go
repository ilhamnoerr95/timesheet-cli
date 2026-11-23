package cli

import (
	"fmt"
	"io"
)

var scripts = map[string]string{
	"bash": `_timesheet() {
  local cur=${COMP_WORDS[COMP_CWORD]} prev=${COMP_WORDS[COMP_CWORD-1]}
  if [ "$COMP_CWORD" -eq 1 ]; then
    COMPREPLY=($(compgen -W "add list rm export sheet completion help" -- "$cur"))
  elif [ "$prev" = completion ]; then
    COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur"))
  else
    COMPREPLY=($(compgen -W "--date --month --to --in --out --place --task --sheet --yes --force" -- "$cur"))
  fi
}
complete -F _timesheet timesheet
`,
	"zsh": `#compdef timesheet
_timesheet() {
  local -a cmds
  cmds=('add:fill in a day' 'list:show the month' 'sheet:link or switch timesheet files' 'rm:clear a day' 'export:write CSV' 'completion:print shell completion' 'help:show help')
  if (( CURRENT == 2 )); then
    _describe command cmds
  elif [[ $words[2] == completion ]]; then
    _values shell bash zsh fish
  else
    _arguments '--date[YYYY-MM-DD]:date:' '--month[YYYY-MM]:month:' '--to[CSV file]:file:_files' '--in[check in]:time:' '--out[check out]:time:' '--place[work place]:place:' '--task[task]:task:' '--sheet[timesheet name]:sheet:' '--yes[skip confirmation]' '--force[ignore lock file]'
  fi
}
compdef _timesheet timesheet
`,
	"fish": `complete -c timesheet -f
complete -c timesheet -n __fish_use_subcommand -a 'add list rm export sheet completion help'
complete -c timesheet -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
complete -c timesheet -l date -d 'YYYY-MM-DD'
complete -c timesheet -l month -d 'YYYY-MM'
complete -c timesheet -l to -r -d 'CSV file'
complete -c timesheet -l in -r -d 'check in'
complete -c timesheet -l out -r -d 'check out'
complete -c timesheet -l place -r -d 'work place'
complete -c timesheet -l task -r -d 'task'
complete -c timesheet -l sheet -r -d 'timesheet name'
complete -c timesheet -l yes -d 'skip confirmation'
complete -c timesheet -l force -d 'ignore lock file'
`,
}

func completion(pos []string, out io.Writer) error {
	script, ok := scripts[first(pos)]
	if !ok {
		return usageError("completion needs one of: bash, zsh, fish")
	}
	_, err := fmt.Fprint(out, script)
	return err
}
