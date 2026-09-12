package interactive

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"charm.land/huh/v2"
)

var ErrCancelled = errors.New("interactive session cancelled")

type Option struct {
	Label string
	Value string
}

type Prompter interface {
	SetLanguage(language string)
	Select(title, description string, options []Option, defaultValue string) (string, error)
	Input(title, description, defaultValue string, validate func(string) error) (string, error)
	Confirm(title, description string, defaultValue bool) (bool, error)
}

type HuhPrompter struct {
	InputReader io.Reader
	Output      io.Writer
	Accessible  bool
	Language    string
	reader      *bufio.Reader
}

func NewHuhPrompter(input io.Reader, output io.Writer) *HuhPrompter {
	if input == nil {
		input = os.Stdin
	}
	if output == nil {
		output = os.Stderr
	}
	return &HuhPrompter{
		InputReader: input,
		Output:      output,
		Accessible:  os.Getenv("ACCESSIBLE") != "",
		Language:    "en",
		reader:      bufio.NewReader(input),
	}
}

func (prompt *HuhPrompter) SetLanguage(language string) {
	prompt.Language = normalizeLanguage(language)
}

func (prompt *HuhPrompter) Select(title, description string, options []Option, defaultValue string) (string, error) {
	if prompt.Accessible {
		return prompt.lineSelect(title, description, options, defaultValue)
	}
	value := defaultValue
	huhOptions := make([]huh.Option[string], 0, len(options))
	for _, option := range options {
		huhOptions = append(huhOptions, huh.NewOption(option.Label, option.Value).Selected(option.Value == defaultValue))
	}
	field := huh.NewSelect[string]().Title(title).Description(description).Options(huhOptions...).Value(&value)
	if err := prompt.form(field).Run(); err != nil {
		return "", normalizePromptError(err)
	}
	return value, nil
}

func (prompt *HuhPrompter) Input(title, description, defaultValue string, validate func(string) error) (string, error) {
	if prompt.Accessible {
		return prompt.lineInput(title, description, defaultValue, validate)
	}
	value := defaultValue
	field := huh.NewInput().Title(title).Description(description).Value(&value)
	if validate != nil {
		field.Validate(validate)
	}
	if err := prompt.form(field).Run(); err != nil {
		return "", normalizePromptError(err)
	}
	return value, nil
}

func (prompt *HuhPrompter) Confirm(title, description string, defaultValue bool) (bool, error) {
	if prompt.Accessible {
		return prompt.lineConfirm(title, description, defaultValue)
	}
	value := defaultValue
	field := huh.NewConfirm().Title(title).Description(description).Affirmative(text(prompt.Language, "common.yes")).Negative(text(prompt.Language, "common.no")).Value(&value)
	if err := prompt.form(field).Run(); err != nil {
		return false, normalizePromptError(err)
	}
	return value, nil
}

func (prompt *HuhPrompter) form(field huh.Field) *huh.Form {
	form := huh.NewForm(huh.NewGroup(field))
	if prompt.InputReader != nil {
		form.WithInput(prompt.InputReader)
	}
	if prompt.Output != nil {
		form.WithOutput(prompt.Output)
	}
	return form
}

func (prompt *HuhPrompter) lineSelect(title, description string, options []Option, defaultValue string) (string, error) {
	if len(options) == 0 {
		return "", errors.New("interactive select has no options")
	}
	prompt.heading(title, description)
	defaultIndex := 0
	for index, option := range options {
		fmt.Fprintf(prompt.Output, "  %d. %s\n", index+1, option.Label)
		if option.Value == defaultValue {
			defaultIndex = index
		}
	}
	for {
		fmt.Fprintf(prompt.Output, text(prompt.Language, "accessible.choose"), defaultIndex+1)
		value, err := prompt.readLine()
		if err != nil {
			return "", err
		}
		if value == "" {
			return options[defaultIndex].Value, nil
		}
		index, parseErr := strconv.Atoi(value)
		if parseErr == nil && index >= 1 && index <= len(options) {
			return options[index-1].Value, nil
		}
		fmt.Fprintln(prompt.Output, text(prompt.Language, "accessible.invalid_choice", len(options)))
	}
}

func (prompt *HuhPrompter) lineInput(title, description, defaultValue string, validate func(string) error) (string, error) {
	prompt.heading(title, description)
	for {
		if defaultValue == "" {
			fmt.Fprint(prompt.Output, "> ")
		} else {
			fmt.Fprintf(prompt.Output, text(prompt.Language, "accessible.input_default"), defaultValue)
		}
		value, err := prompt.readLine()
		if err != nil {
			return "", err
		}
		if value == "" {
			value = defaultValue
		}
		if validate != nil {
			if validateErr := validate(value); validateErr != nil {
				fmt.Fprintln(prompt.Output, validateErr)
				continue
			}
		}
		return value, nil
	}
}

func (prompt *HuhPrompter) lineConfirm(title, description string, defaultValue bool) (bool, error) {
	prompt.heading(title, description)
	suffix := " [y/N]: "
	if defaultValue {
		suffix = " [Y/n]: "
	}
	for {
		fmt.Fprint(prompt.Output, suffix)
		value, err := prompt.readLine()
		if err != nil {
			return false, err
		}
		if value == "" {
			return defaultValue, nil
		}
		switch strings.ToLower(value) {
		case "y", "yes", "д", "да":
			return true, nil
		case "n", "no", "н", "нет":
			return false, nil
		default:
			fmt.Fprintln(prompt.Output, text(prompt.Language, "accessible.yes_no"))
		}
	}
}

func (prompt *HuhPrompter) heading(title, description string) {
	fmt.Fprintf(prompt.Output, "\n%s\n", title)
	if strings.TrimSpace(description) != "" {
		fmt.Fprintln(prompt.Output, description)
	}
}

func (prompt *HuhPrompter) readLine() (string, error) {
	value, err := prompt.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.TrimSpace(value)
	if errors.Is(err, io.EOF) && value == "" {
		return "", ErrCancelled
	}
	return value, nil
}

func normalizePromptError(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCancelled
	}
	return err
}
