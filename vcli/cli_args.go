package vcli

import (
	"flag"
	"fmt"
	"github.com/c-bata/go-prompt"
	"os"
	"strings"
)

type CliArgs struct {
	image       *string
	revision    *string
	memsizeMib  *uint
	vCpuCount   *uint
	amount      *int
	id          *string
	all         *bool
}

const (
	defaultMemSize = 256
	defaultVCPUCount = 1
	defaultAmount = 1
)

func newCliArgs() *CliArgs {
	c := &CliArgs{
		image: flag.String("image", "", "Image name to boot a uVM from (when booting from scratch)"),
		revision: flag.String("revision", "", "Revision id (eg. helloworld-go-00001)"),
		memsizeMib: flag.Uint("memsizemib", defaultMemSize, "Capacity set aside for storing snapshots (Mib)"),
		vCpuCount:   flag.Uint("vcpucount", defaultVCPUCount, "Capacity set aside for storing snapshots (Mib)"),
		amount: flag.Int("amount", defaultAmount, "Turn on to boot from snapshot"),
		id: flag.String("id", "", "Id to delete"),
		all: flag.Bool("all", false, "Delete all vms"),
	}

	return c
}

func (c *CliArgs) parseArgs(fields []string) {
	os.Args = append([]string{"cmd"}, fields...)
	flag.Parse()
}

func (c *CliArgs) setDefaults() {
	if err := flag.Set("image", ""); err != nil {
		fmt.Println(err)
	}
	if err := flag.Set("revision", ""); err != nil {
		fmt.Println(err)
	}
	if err := flag.Set("memsizemib", fmt.Sprintf("%d", defaultMemSize)); err != nil {
		fmt.Println(err)
	}
	if err := flag.Set("vcpucount", fmt.Sprintf("%d", defaultVCPUCount)); err != nil {
		fmt.Println(err)
	}
	if err := flag.Set("amount", fmt.Sprintf("%d", defaultAmount)); err != nil {
		fmt.Println(err)
	}
	if err := flag.Set("id", ""); err != nil {
		fmt.Println(err)
	}
	if err := flag.Set("all", "false"); err != nil {
		fmt.Println(err)
	}
}

func (c *CliArgs) checkCreateFlags() bool {
	if *c.image == "" {
		fmt.Println("Incorrect usage. 'revision' needs to be specified")
		return false
	}

	if *c.revision == "" {
		fmt.Println("Incorrect usage. 'revision' needs to be specified")
		return false
	}
	return true
}

func (c *CliArgs) checkDeleteFlags() bool {
	counter := 0
	if *c.id != "" {
		counter += 1
	}
	if *c.image != "" {
		counter += 1
	}
	if *c.revision != "" {
		counter += 1
	}
	if *c.all {
		counter += 1
	}

	if counter == 0 {
		fmt.Println("Incorrect usage. Please specify a delete option")
		return false
	}

	if counter > 1 {
		fmt.Println("Incorrect usage. Please only use a single delete option at a time")
		return false
	}
	return true
}

func (c *CliArgs) getCompleter(vmControl *VmController) func(in prompt.Document) []prompt.Suggest {
	return func(in prompt.Document) []prompt.Suggest {
		vmControl.safeLock.Lock()
		defer vmControl.safeLock.Unlock()
		uVms := vmControl.safeUvms

		curLine := in.CurrentLine()
		fields := strings.Fields(curLine)

		var suggestions []prompt.Suggest

		switch {
		case len(fields) == 0 || len(fields) == 1 && curLine[len(curLine)-1:] != " ":
			suggestions = firstSuggest
		case fields[0] == "create":
			suggestions = createSuggest
		case fields[0] == "delete" && ((len(fields) > 1 && fields[len(fields)-1] == "-id") || (len(fields) > 2 && fields[len(fields)-2] == "-id")):
			suggestions = getUVmSuggestions(uVms)
		case fields[0] == "delete" && ((len(fields) > 1 && fields[len(fields)-1] == "-image") || (len(fields) > 2 && fields[len(fields)-2] == "-image")):
			suggestions = getImageSuggestions(uVms)
		case fields[0] == "delete" && ((len(fields) > 1 && fields[len(fields)-1] == "-revision") || (len(fields) > 2 && fields[len(fields)-2] == "-revision")):
			suggestions = getRevisionSuggestions(uVms)
		case fields[0] == "delete":
			suggestions = deleteSuggest
		default:
			suggestions = []prompt.Suggest {}
		}

		return prompt.FilterHasPrefix(suggestions, in.GetWordBeforeCursor(), false)
	}
}

// Completions
var firstSuggest = []prompt.Suggest {
	{Text: "ls", Description: "List all running uVMs"},
	{Text: "create", Description: "Create a uVM"},
	{Text: "delete", Description: "Delete a uVM"},
}

var createSuggest = []prompt.Suggest {
	{Text: "-image", Description: "Image to boot vm from"},
	{Text: "-revision", Description: "Revision the vm should be part of (for snapshots)"},
	{Text: "-memsizemib", Description: "Memory size for the vm"},
	{Text: "-vcpucount", Description: "Memory size for the vm"},
	{Text: "-amount", Description: "Amount of instances to create"},
}

var deleteSuggest = []prompt.Suggest {
	{Text: "-id", Description: "Delete vm with given id"},
	{Text: "-image", Description: "Delete all vms with given image"},
	{Text: "-revision", Description: "Delete all vms with given revision"},
	{Text: "-all", Description: "Delete all vms"},
}

func getUVmSuggestions(uVms map[string]*VmInstance) []prompt.Suggest {
	suggestions := make([]prompt.Suggest, len(uVms))

	i := 0
	for k, vmInstance := range uVms {
		suggestions[i].Text = k
		suggestions[i].Description = fmt.Sprintf("id %s, rev %s, mem %d", vmInstance.containerID, vmInstance.revisionId, vmInstance.memSizeMib)
		i += 1
	}
	return suggestions
}

func getImageSuggestions(uVms map[string]*VmInstance) []prompt.Suggest {
	seen := map[string]int{}

	for _, vmInstance := range uVms {
		image := vmInstance.image
		if _, present := seen[image]; !present {
			seen[image] = 1
		} else {
			seen[image] += 1
		}
	}

	suggestions := make([]prompt.Suggest, len(seen))
	i := 0
	for image, count := range seen {
		suggestions[i].Text = image
		suggestions[i].Description = fmt.Sprintf("%d instances", count)
		i += 1
	}

	return suggestions
}

func getRevisionSuggestions(uVms map[string]*VmInstance) []prompt.Suggest {
	seen := map[string]int{}

	for _, vmInstance := range uVms {
		revision := vmInstance.revisionId
		if _, present := seen[revision]; !present {
			seen[revision] = 1
		} else {
			seen[revision] += 1
		}
	}

	suggestions := make([]prompt.Suggest, len(seen))
	i := 0
	for image, count := range seen {
		suggestions[i].Text = image
		suggestions[i].Description = fmt.Sprintf("%d instances", count)
		i += 1
	}

	return suggestions
}