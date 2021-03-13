package interactive

import (
	"bufio"
	"fmt"
	"github.com/ffuf/ffuf/pkg/ffuf"
	"github.com/ffuf/ffuf/pkg/filter"
	"strconv"
	"strings"
	"time"
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
			i.Job.Reset()
			i.paused = false
			i.Job.Output.Info("Restarting the current ffuf job!")
			i.Job.Resume()
		case "show":
			for _, r := range i.Job.Output.GetResults() {
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
				i.updateFilter("status", args[1])
				i.Job.Output.Info("New status code filter value set")
			}
		case "fl":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value for line count filter, or \"none\" for removing it")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"fl\"")
			} else {
				i.updateFilter("line", args[1])
				i.Job.Output.Info("New line count filter value set")
			}
		case "fw":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value for word count filter, or \"none\" for removing it")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"fw\"")
			} else {
				i.updateFilter("word", args[1])
				i.Job.Output.Info("New word count filter value set")
			}
		case "fs":
			if len(args) < 2 {
				i.Job.Output.Error("Please define a value for response size filter, or \"none\" for removing it")
			} else if len(args) > 2 {
				i.Job.Output.Error("Too many arguments for \"fs\"")
			} else {
				i.updateFilter("size", args[1])
				i.Job.Output.Info("New response size filter value set")
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

func (i *interactive) updateFilter(name, value string) {
	if value == "none" {
		filter.RemoveFilter(i.Job.Config, name)
	} else {
		newFc, err := filter.NewFilterByName(name, value)
		if err != nil {
			i.Job.Output.Error(fmt.Sprintf("Error while setting new filter value: %s", err))
			return
		} else {
			i.Job.Config.Filters[name] = newFc
		}

		results := make([]ffuf.Result, 0)
		for _, res := range i.Job.Output.GetResults() {
			fakeResp := &ffuf.Response{
				StatusCode:    res.StatusCode,
				ContentLines:  res.ContentLength,
				ContentWords:  res.ContentWords,
				ContentLength: res.ContentLength,
			}
			filterOut, _ := newFc.Filter(fakeResp)
			if !filterOut {
				results = append(results, res)
			}
		}
		i.Job.Output.SetResults(results)
	}
}

func (i *interactive) printQueue() {
	if len(i.Job.QueuedJobs()) > 0 {
		i.Job.Output.Raw("Queued recursion jobs:\n")
		for index, job := range i.Job.QueuedJobs() {
			postfix := ""
			if index == 0 {
				postfix = " (active job)"
			}
			i.Job.Output.Raw(fmt.Sprintf(" [%d] : %s%s\n", index, job.Url, postfix))
		}
	} else {
		i.Job.Output.Info("Recursion job queue is empty")
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
			i.Job.Output.Info("Recursion job successfully deleted!")
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
	var fc, fl, fs, fw string
	for name, filter := range i.Job.Config.Filters {
		switch name {
		case "status":
			fc = "(active: " + filter.Repr() + ")"
		case "line":
			fl = "(active: " + filter.Repr() + ")"
		case "word":
			fw = "(active: " + filter.Repr() + ")"
		case "size":
			fs = "(active: " + filter.Repr() + ")"
		}
	}
	help := `
available commands:
 fc [value]             - (re)configure status code filter %s
 fl [value]             - (re)configure line count filter %s
 fw [value]             - (re)configure word count filter %s
 fs [value]             - (re)configure size filter %s
 queueshow              - show recursive job queue
 queuedel [number]      - delete a recursion job in the queue
 queueskip              - advance to the next queued recursion job
 restart                - restart and resume the current ffuf job
 resume                 - resume current ffuf job (or: ENTER) 
 show                   - show results
 savejson [filename]    - save current matches to a file
 help                   - you are looking at it
`
	i.Job.Output.Raw(fmt.Sprintf(help, fc, fl, fw, fs))
}
