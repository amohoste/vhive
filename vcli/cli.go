package vcli

import (
	"github.com/c-bata/go-prompt"
	fccdcri "github.com/ease-lab/vhive/cri"
	"strings"
)

var (
	args         *CliArgs
	vmControl    *VmController
)

func deleteVms() {
	if *args.all {
		vmControl.deleteAll()
	} else {
		if *args.id != "" {
			vmControl.delete(*args.id)
		}
		if *args.revision != "" {
			vmControl.deleteByRevision(*args.revision)
		}
		if *args.image != "" {
			vmControl.deleteByImage(*args.image)
		}
	}
}

func createVms() {
	for i := 0; i < *args.amount; i++ {
		vmControl.create(*args.image, *args.revision, *args.memsizeMib, *args.vCpuCount)
	}
}

func executor(input string) {
	args.setDefaults()
	fields := strings.Fields(input)
	if len(fields) > 0 {
		args.parseArgs(strings.Fields(input)[1:])
		switch fields[0] {
		case "ls":
			vmControl.list()
		case "create":
			if args.checkCreateFlags() {
				createVms()
			}
		case "delete":
			if args.checkDeleteFlags() {
				deleteVms()
			}
		}
	}
}

func CreateCli(coordinator *fccdcri.Coordinator) {
//func main() {
	args = newCliArgs()
	vmControl = newVmController(coordinator) // vmControl = newVmController()

	p := prompt.New(
		executor,
		args.getCompleter(vmControl),
		prompt.OptionPrefix("> vhive-control "),
		prompt.OptionShowCompletionAtStart(),
		// Option + arrow skip word
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{0x1b, 0x62},
			Fn:        prompt.GoLeftWord,
		}),
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{0x1b, 0x66},
			Fn:        prompt.GoRightWord,
		}),
		// Option + delete to delete word
		prompt.OptionAddASCIICodeBind(prompt.ASCIICodeBind{
			ASCIICode: []byte{0x1b, 0x7f},
			Fn:        prompt.DeleteWord,
		}),
	)

	p.Run()
}