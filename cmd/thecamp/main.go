package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/hallazzang/thecamp"
	"github.com/manifoldco/promptui"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	client      *thecamp.Client
	group       *thecamp.Group
	traineeInfo *thecamp.TraineeInfo
)

func readInput(prompt string, echo bool) (string, error) {
	fmt.Print(prompt)
	if echo {
		r := bufio.NewReader(os.Stdin)
		s, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSuffix(s, "\n"), nil
	} else {
		b, err := terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return "", err
		}
		fmt.Println()
		return string(b), nil
	}
}

func showTrainee() {
	if traineeInfo == nil {
		fmt.Println(promptui.IconWarn, "No group is selected. Select group first")
		return
	}

	cyan := promptui.Styler(promptui.FGCyan)
	fmt.Println(cyan("Name:"), traineeInfo.Name)
	fmt.Println(cyan("Birthday:"), traineeInfo.Birthday)
	fmt.Println(cyan("Relationship:"), traineeInfo.Relationship)
}

func selectGroup() {
	groups, err := client.Groups()
	if err != nil {
		panic(err)
	}

	p := promptui.Select{
		Label: "Group",
		Items: groups,
		Templates: &promptui.SelectTemplates{
			Label:    `{{ "Select" | yellow | bold }} {{ . | white | bold}}`,
			Active:   `{{ "->" | green | bold }} {{ .FullName }}`,
			Inactive: `   {{ .FullName }}`,
			Selected: `{{ "Selected:" | faint }} {{ .FullName | faint }}`,
			Details: `
{{ "Selection Details" | cyan | bold }}
{{ "Unit:" | faint }} {{ .UnitName }}
{{ "Name:" | faint }} {{ .Name }}
{{ "Entered Date:" | faint }} {{ .EnteredDate }}`,
			Help: `{{ "(Use arrow keys to navigate)" | faint }}`,
		},
	}
	i, _, err := p.Run()
	if err != nil {
		panic(err)
	}
	group = groups[i]

	ti, err := client.TraineeInfo(group)
	if err != nil {
		panic(err)
	}

	traineeInfo = ti
}

func sendLetter() {
	aborted, title, err := inputPrompt("Title", false)
	if err != nil {
		panic(err)
	} else if aborted {
		return
	}

	aborted, content, err := inputPrompt("Content", false)
	if err != nil {
		panic(err)
	} else if aborted {
		return
	}

	suc, err := client.SendLetter(traineeInfo, title, content)
	if err != nil {
		panic(err)
	}

	if suc {
		fmt.Println(promptui.IconGood, "Successfully sent the letter to trainee")
	} else {
		fmt.Println(promptui.IconWarn, "Failed to send letter")
	}
}

func listLetters() {
	if group == nil {
		fmt.Println(promptui.IconWarn, "No group is selected. Select group first")
		return
	}

	lit := client.LettersIterator(group, thecamp.Ascending)

	fmt.Println("Last 10 letters:")
	for i := 1; i <= 10; i++ {
		cont, err := lit.Next()
		if err != nil {
			panic(err)
		} else if !cont {
			break
		}

		letter := lit.Letter()
		fmt.Println(promptui.Styler(promptui.FGCyan)(fmt.Sprintf("%2d", i)), letter.Title)
	}
}

func inputPrompt(label string, masked bool) (aborted bool, value string, err error) {
	var mask rune
	if masked {
		mask = '*'
	}
	p := promptui.Prompt{
		Label: label,
		Templates: &promptui.PromptTemplates{
			Valid:   `{{ "Input" | yellow | bold }} {{ . | white | bold }}: `,
			Success: `{{ . | faint | bold }}: `,
		},
		Mask: mask,
	}
	value, err = p.Run()
	if err == io.EOF || err == promptui.ErrEOF || err == promptui.ErrInterrupt {
		fmt.Println(promptui.IconBad, "Aborted")
		aborted = true
		err = nil
	}
	return
}

func main() {
	var err error

	client, err = thecamp.NewClient()
	if err != nil {
		panic(err)
	}

	aborted, id, err := inputPrompt("ID", false)
	if err != nil {
		panic(err)
	} else if aborted {
		return
	}

	aborted, pw, err := inputPrompt("PW", true)
	if err != nil {
		panic(err)
	} else if aborted {
		return
	}

	suc, err := client.Login(id, pw)
	if err != nil {
		panic(err)
	} else if !suc {
		fmt.Println("login failed")
		return
	}

loop:
	for {
		selections := []struct {
			Label       string
			Description string
		}{
			{"Show Trainee", "Show trainee information of currently selected group"},
			{"Select Group", "Select new group for sending letter to"},
			{"Send Letter", "Send a letter to trainee in selected group"},
			{"List Letters", "List letters that have been sent"},
			{"Quit", "Quit the program"},
		}

		p := promptui.Select{
			Label: "Action",
			Items: selections,
			Templates: &promptui.SelectTemplates{
				Label:    `{{ "Select" | yellow | bold }} {{ . | white | bold }}`,
				Active:   `{{ "->" | green | bold}} {{ .Label }}`,
				Inactive: `   {{ .Label }}`,
				Selected: `{{ "Selected:" | faint }} {{ .Label | faint}}`,
				Details: `
{{ "Description" | cyan | bold }}
{{ .Description }}`,
				Help: `{{ "(Use arrow keys to navigate)" | faint }}`,
			},
		}
		i, _, err := p.Run()
		if err == io.EOF || err == promptui.ErrEOF || err == promptui.ErrInterrupt {
			fmt.Println(promptui.IconBad, "Aborted")
			break loop
		} else if err != nil {
			panic(err)
		}

		switch i {
		case 0:
			showTrainee()
		case 1:
			selectGroup()
		case 2:
			sendLetter()
		case 3:
			listLetters()
		case 4:
			break loop
		}
	}
}
