package interactive

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type interactive struct {
	Job    *ffuf.Job
	paused bool
}

func Handle(job *ffuf.Job) error {
	i := interactive{job, false}
	tty, err := termHandle()
	if err != nil {
		return err
	}
	defer tty.Close()
	inreader := bufio.NewScanner(tty)
	inreader.Split(bufio.ScanLines)
	for inreader.Scan() {
		i.handleInput(inreader.Bytes())
	}
	return nil
}

func (i *interactive) handleInput(in []byte) {
	instr := string(in)
	args := strings.Split(strings.TrimSpace(instr), " ")
	if len(args) == 1 && args[0] == "" {
		// Enter pressed - toggle interactive state
		i.paused = !i.paused
		if i.paused {
			i.Job.Pause()
			time.Sleep(500 * time.Millisecond)
			i.printBanner()
		} else {
			i.Job.Resume()
		}
	} else {
		switch args[0] {
		case "?":
			i.printHelp()
		case "help":
			i.printHelp()
		case "resume":
			i.paused = false
			i.Job.Resume()
		case "restart":
			i.Job.Reset(false)
			i.paused = false
			i.Job.Output.Info("Restarting the current ffuf job!")
			i.Job.Resume()
		case "show":
			for _, r := range i.Job.Output.GetCurrentResults() {
				i.Job.Output.PrintResult(r)
			}
		case "savejson":
			if len(args) < 2 {
				i.Job.Output.Error("Please define the filename")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"savejson\"")
			} else {
				err := i.Job.Output.SaveFile(args[1], "json")
				if err != nil {
					i.Job.Output.Error(fmt.Sprintf("%s", err))
				} else {
					i.Job.Output.Info("Output file successfully saved!")
				}
			}
		case "fc":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value for status code filter, or \"none\" for removing it")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"fc\"")
			} else {
				i.updateFilter("status", args[1], true)
				i.Job.Output.Info("New status code filter value set")
			}
		case "afc":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value to append to status code filter")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"afc\"")
			} else {
				i.appendFilter("status", args[1])
				i.Job.Output.Info("New status code filter value set")
			}
		case "fl":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value for line count filter, or \"none\" for removing it")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"fl\"")
			} else {
				i.updateFilter("line", args[1], true)
				i.Job.Output.Info("New line count filter value set")
			}
		case "afl":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value to append to line count filter")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"afl\"")
			} else {
				i.appendFilter("line", args[1])
				i.Job.Output.Info("New line count filter value set")
			}
		case "fw":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value for word count filter, or \"none\" for removing it")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"fw\"")
			} else {
				i.updateFilter("word", args[1], true)
				i.Job.Output.Info("New word count filter value set")
			}
		case "afw":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value to append to word count filter")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"afw\"")
			} else {
				i.appendFilter("word", args[1])
				i.Job.Output.Info("New word count filter value set")
			}
		case "fs":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value for response size filter, or \"none\" for removing it")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"fs\"")
			} else {
				i.updateFilter("size", args[1], true)
				i.Job.Output.Info("New response size filter value set")
			}
		case "afs":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value to append to size filter")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"afs\"")
			} else {
				i.appendFilter("size", args[1])
				i.Job.Output.Info("New response size filter value set")
			}
		case "ft":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value for response time filter, or \"none\" for removing it")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"ft\"")
			} else {
				i.updateFilter("time", args[1], true)
				i.Job.Output.Info("New response time filter value set")
			}
		case "aft":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value to append to response time filter")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"aft\"")
			} else {
				i.appendFilter("time", args[1])
				i.Job.Output.Info("New response time filter value set")
			}
		case "queueshow":
			i.printQueue()
		case "queuedel":
			if len(args) < 2 {
				i.Job.Output.Error("Please define the index of a queued job to remove. Use \"queueshow\" for listing of jobs.")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"queuedel\"")
			} else {
				i.deleteQueue(args[1])
			}
		case "queueskip":
			i.Job.SkipQueue()
			i.Job.Output.Info("Skipping to the next queued job")
		default:
			if i.paused {
				i.Job.Output.Warning(fmt.Sprintf("Unknown command: \"%s\". Enter \"help\" for a list of available commands", args[0]))
			} else {
				i.Job.Output.Error("NOPE")
			}
		}
	}

	if i.paused {
		i.printPrompt()
	}
}

func (i *interactive) refreshResults() {
	results := make([]ffuf.Result, 0)
	filters := i.Job.Config.MatcherManager.GetFilters()
	for _, filter := range filters {
		for _, res := range i.Job.Output.GetCurrentResults() {
			fakeResp := &ffuf.Response{
				StatusCode:    res.StatusCode,
				ContentLines:  res.ContentLength,
				ContentWords:  res.ContentWords,
				ContentLength: res.ContentLength,
			}
			filterOut, _ := filter.Filter(fakeResp)
			if !filterOut {
				results = append(results, res)
			}
		}
	}
	i.Job.Output.SetCurrentResults(results)
}

func (i *interactive) updateFilter(name, value string, replace bool) {
	if value == "none" {
		i.Job.Config.MatcherManager.RemoveFilter(name)
	} else {
		_ = i.Job.Config.MatcherManager.AddFilter(name, value, replace)
	}
	i.refreshResults()
}

func (i *interactive) appendFilter(name, value string) {
	i.updateFilter(name, value, false)
}

func (i *interactive) printQueue() {
	if len(i.Job.QueuedJobs()) > 0 {
		i.Job.Output.Raw("Queued jobs:\n")
		for index, job := range i.Job.QueuedJobs() {
			postfix := ""
			if index == 0 {
				postfix = " (active job)"
			}
			i.Job.Output.Raw(fmt.Sprintf(" [%d] : %s%s\n", index, job.Url, postfix))
		}
	} else {
		i.Job.Output.Info("Job queue is empty")
	}
}

func (i *interactive) deleteQueue(in string) {
	index, err := strconv.Atoi(in)
	if err != nil {
		i.Job.Output.Warning(fmt.Sprintf("Not a number: %s", in))
	} else {
		if index < 0 || index > len(i.Job.QueuedJobs())-1 {
			i.Job.Output.Warning("No such queued job. Use \"queueshow\" to list the jobs in queue")
		} else if index == 0 {
			i.Job.Output.Warning("Cannot delete the currently running job. Use \"queueskip\" to advance to the next one")
		} else {
			i.Job.DeleteQueueItem(index)
			i.Job.Output.Info("Job successfully deleted!")
		}
	}
}
func (i *interactive) printBanner() {
	i.Job.Output.Raw("entering interactive mode\ntype \"help\" for a list of commands, or ENTER to resume.\n")
}

func (i *interactive) printPrompt() {
	i.Job.Output.Raw("> ")
}

func (i *interactive) printHelp() {
	var fc, fl, fs, ft, fw string
	for name, filter := range i.Job.Config.MatcherManager.GetFilters() {
		switch name {
		case "status":
			fc = "(active: " + filter.Repr() + ")"
		case "line":
			fl = "(active: " + filter.Repr() + ")"
		case "word":
			fw = "(active: " + filter.Repr() + ")"
		case "size":
			fs = "(active: " + filter.Repr() + ")"
		case "time":
			ft = "(active: " + filter.Repr() + ")"
		}
	}
	help := `
available commands:
 afc [value]             - append to status code filter %s
 fc  [value]             - (re)configure status code filter %s
 afl [value]             - append to line count filter %s
 fl  [value]             - (re)configure line count filter %s
 afw [value]             - append to word count filter %s
 fw  [value]             - (re)configure word count filter %s
 afs [value]             - append to size filter %s
 fs  [value]             - (re)configure size filter %s
 aft [value]             - append to time filter %s
 ft  [value]			 - (re)configure time filter %s
 queueshow              - show job queue
 queuedel [number]      - delete a job in the queue
 queueskip              - advance to the next queued job
 restart                - restart and resume the current ffuf job
 resume                 - resume current ffuf job (or: ENTER) 
 show                   - show results for the current job
 savejson [filename]    - save current matches to a file
 help                   - you are looking at it
`
	i.Job.Output.Raw(fmt.Sprintf(help, fc, fc, fl, fl, fw, fw, fs, fs, ft, ft))
}
