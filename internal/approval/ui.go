package approval

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/krishyogee/gitmate/internal/tui"
)

type UI interface {
	Prompt(card Card) (Decision, string, error)
}

type TerminalUI struct {
	In  io.Reader
	Out io.Writer
}

var (
	stdinReaderOnce sync.Once
	stdinReader     *bufio.Reader
)

func sharedStdinReader() *bufio.Reader {
	stdinReaderOnce.Do(func() {
		stdinReader = bufio.NewReader(os.Stdin)
	})
	return stdinReader
}

func SharedStdin() *bufio.Reader { return sharedStdinReader() }

func (t *TerminalUI) reader() *bufio.Reader {
	if t.In == nil {
		return sharedStdinReader()
	}
	return bufio.NewReader(t.In)
}

func (t *TerminalUI) out() io.Writer {
	if t.Out == nil {
		return os.Stdout
	}
	return t.Out
}

func (t *TerminalUI) Prompt(card Card) (Decision, string, error) {
	w := t.out()
	r := t.reader()

	if tui.IsTTY() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, tui.RenderApprovalCard(tui.ApprovalView{
			Action:      card.Action,
			Risk:        card.Risk.String(),
			Description: card.Description,
			Input:       card.Input,
			Preview:     card.Preview,
		}))
	} else {
		t.renderCard(w, card)
	}

	for {
		if tui.IsTTY() {
			fmt.Fprintln(w)
			fmt.Fprintln(w, tui.RenderApprovalPrompt())
			fmt.Fprint(w, "› ")
		} else {
			fmt.Fprint(w, "\n[y]es  [a]llow session  [p]review  [e]dit  [n]o  [?]explain  > ")
		}
		line, err := r.ReadString('\n')
		if err != nil {
			return DecisionNo, "", err
		}
		choice := strings.TrimSpace(strings.ToLower(line))
		switch choice {
		case "y", "yes", "":
			return DecisionYes, card.Input, nil
		case "a", "allow":
			return DecisionSession, card.Input, nil
		case "n", "no":
			return DecisionNo, "", nil
		case "p", "preview":
			t.renderPreview(w, card)
			continue
		case "e", "edit":
			edited, err := openEditor(card.Input)
			if err != nil {
				fmt.Fprintf(w, "edit failed: %v\n", err)
				continue
			}
			return DecisionEdit, edited, nil
		case "?", "explain":
			t.renderExplain(w, card)
			continue
		default:
			fmt.Fprintln(w, "unknown choice")
		}
	}
}

func (t *TerminalUI) renderCard(w io.Writer, card Card) {
	bar := strings.Repeat("─", 47)
	fmt.Fprintf(w, "\n╭%s╮\n", bar)
	fmt.Fprintf(w, "│ gitmate — Action Required%s│\n", strings.Repeat(" ", 47-len("gitmate — Action Required")-1))
	fmt.Fprintf(w, "├%s┤\n", bar)
	t.padLine(w, fmt.Sprintf("Action:  %s", card.Action))
	t.padLine(w, fmt.Sprintf("Risk:    %s", card.Risk.String()))
	if card.Description != "" {
		t.padLine(w, fmt.Sprintf("Why:     %s", card.Description))
	}
	fmt.Fprintf(w, "╰%s╯\n", bar)
	if card.Input != "" {
		fmt.Fprintln(w, "─── input ───")
		fmt.Fprintln(w, card.Input)
		fmt.Fprintln(w, "─────────────")
	}
}

func (t *TerminalUI) padLine(w io.Writer, line string) {
	width := 47
	runes := []rune(line)
	max := width - 2
	if len(runes) > max {
		runes = append(runes[:max-1], '…')
	}
	line = string(runes)
	pad := width - len(runes) - 1
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(w, "│ %s%s│\n", line, strings.Repeat(" ", pad))
}

func (t *TerminalUI) renderPreview(w io.Writer, card Card) {
	if card.Preview != "" {
		fmt.Fprintln(w, "\n─── preview ───")
		fmt.Fprintln(w, card.Preview)
		fmt.Fprintln(w, "───────────────")
		return
	}
	fmt.Fprintln(w, "\n─── input ───")
	fmt.Fprintln(w, card.Input)
	fmt.Fprintln(w, "─────────────")
}

func (t *TerminalUI) renderExplain(w io.Writer, card Card) {
	fmt.Fprintf(w, "\nAction %q is at risk tier %s.\n", card.Action, card.Risk.String())
	switch card.Risk {
	case READ:
		fmt.Fprintln(w, "READ: only reads repo state. No file changes.")
	case ADVISE:
		fmt.Fprintln(w, "ADVISE: generates text. No file changes.")
	case PROPOSE:
		fmt.Fprintln(w, "PROPOSE: prepares a patch but waits for your approval.")
	case EXECUTE:
		fmt.Fprintln(w, "EXECUTE: runs subprocess or writes files. Approval each time.")
	}
}

func openEditor(initial string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	tmp, err := os.CreateTemp("", "gitmate-edit-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(initial); err != nil {
		return "", err
	}
	tmp.Close()

	cmd := exec.Command(editor, tmp.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	data, err := os.ReadFile(tmp.Name())
	if err != nil {
		return "", err
	}
	_ = filepath.Base(tmp.Name())
	return string(data), nil
}
