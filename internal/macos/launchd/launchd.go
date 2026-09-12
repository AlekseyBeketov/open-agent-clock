package launchd

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AlekseyBeketov/open-agent-clock/internal/domain"
)

const labelPrefix = "com.openagentclock.schedule."

var safeID = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

type Spec struct {
	ScheduleID        string
	ProgramArguments  []string
	Interval          time.Duration
	DailyTimes        []string
	StandardOutPath   string
	StandardErrorPath string
	Environment       map[string]string
}

type Status struct {
	Label     string `json:"label"`
	PlistPath string `json:"plist_path"`
	Installed bool   `json:"installed"`
	Loaded    bool   `json:"loaded"`
	Enabled   bool   `json:"enabled"`
	NextRun   string `json:"next_run,omitempty"`
}

type Manager struct {
	LaunchAgentsDir string
	LaunchctlPath   string
	UID             string
}

func NewManager(home string) Manager {
	uid := strconv.Itoa(os.Getuid())
	launchctlPath, err := exec.LookPath("launchctl")
	if err != nil {
		launchctlPath = "/bin/launchctl"
	}
	return Manager{
		LaunchAgentsDir: filepath.Join(home, "Library", "LaunchAgents"),
		LaunchctlPath:   launchctlPath,
		UID:             uid,
	}
}

func Label(scheduleID string) string {
	value := safeID.ReplaceAllString(strings.TrimSpace(scheduleID), "-")
	if value == "" {
		value = "unnamed"
	}
	return labelPrefix + value
}

func (manager Manager) PlistPath(scheduleID string) string {
	return filepath.Join(manager.LaunchAgentsDir, Label(scheduleID)+".plist")
}

func Generate(spec Spec) ([]byte, error) {
	if strings.TrimSpace(spec.ScheduleID) == "" {
		return nil, errors.New("schedule id must not be empty")
	}
	if len(spec.ProgramArguments) == 0 {
		return nil, errors.New("program arguments must not be empty")
	}
	if (spec.Interval > 0) == (len(spec.DailyTimes) > 0) {
		return nil, errors.New("exactly one launchd trigger must be configured")
	}
	job := plist{
		XMLName:           xml.Name{Local: "plist"},
		Version:           "1.0",
		Label:             Label(spec.ScheduleID),
		ProgramArguments:  spec.ProgramArguments,
		StandardOutPath:   spec.StandardOutPath,
		StandardErrorPath: spec.StandardErrorPath,
		Environment:       spec.Environment,
	}
	if spec.Interval > 0 {
		seconds := int(spec.Interval / time.Second)
		if seconds < 1 || spec.Interval != time.Duration(seconds)*time.Second {
			return nil, errors.New("launchd interval must be a whole number of seconds")
		}
		job.StartInterval = &seconds
	} else {
		job.StartCalendarInterval = make([]calendarInterval, 0, len(spec.DailyTimes))
		for _, value := range spec.DailyTimes {
			hour, minute, second, err := parseClock(value)
			if err != nil {
				return nil, err
			}
			job.StartCalendarInterval = append(job.StartCalendarInterval, calendarInterval{Hour: hour, Minute: minute, Second: second})
		}
	}
	var output bytes.Buffer
	output.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	output.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	encoder := xml.NewEncoder(&output)
	encoder.Indent("", "  ")
	if err := encoder.Encode(job); err != nil {
		return nil, fmt.Errorf("encode launchd plist: %w", err)
	}
	output.WriteByte('\n')
	return output.Bytes(), nil
}

func (manager Manager) Install(ctx context.Context, spec Spec) (Status, error) {
	contents, err := Generate(spec)
	if err != nil {
		return Status{}, err
	}
	if err := os.MkdirAll(manager.LaunchAgentsDir, 0o700); err != nil {
		return Status{}, fmt.Errorf("create LaunchAgents directory: %w", err)
	}
	for _, logPath := range []string{spec.StandardOutPath, spec.StandardErrorPath} {
		if strings.TrimSpace(logPath) == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
			return Status{}, fmt.Errorf("create log directory: %w", err)
		}
	}
	path := manager.PlistPath(spec.ScheduleID)
	previous, readErr := os.ReadFile(path)
	wasLoaded := manager.run(ctx, "print", "gui/"+manager.UID+"/"+Label(spec.ScheduleID)) == nil
	if wasLoaded {
		if err := manager.bootout(ctx, spec.ScheduleID); err != nil {
			return Status{}, err
		}
	}
	if err := atomicWrite(path, contents, 0o600); err != nil {
		if wasLoaded {
			_ = manager.bootstrap(ctx, spec.ScheduleID)
		}
		return Status{}, fmt.Errorf("write LaunchAgent plist: %w", err)
	}
	if err := manager.bootstrap(ctx, spec.ScheduleID); err != nil {
		if readErr == nil && len(previous) > 0 {
			_ = atomicWrite(path, previous, 0o600)
			if wasLoaded {
				_ = manager.bootstrap(ctx, spec.ScheduleID)
			}
		} else {
			_ = os.Remove(path)
		}
		return Status{}, fmt.Errorf("load LaunchAgent %s: %w", Label(spec.ScheduleID), err)
	}
	return Status{Label: Label(spec.ScheduleID), PlistPath: path, Installed: true, Loaded: true, Enabled: true}, nil
}

func (manager Manager) Uninstall(ctx context.Context, scheduleID string) error {
	if err := manager.bootout(ctx, scheduleID); err != nil {
		return err
	}
	path := manager.PlistPath(scheduleID)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove LaunchAgent plist: %w", err)
	}
	return nil
}

func (manager Manager) Load(ctx context.Context, scheduleID string) error {
	path := manager.PlistPath(scheduleID)
	if !fileExists(path) {
		return fmt.Errorf("LaunchAgent plist does not exist: %s", path)
	}
	return manager.bootstrap(ctx, scheduleID)
}

func (manager Manager) Unload(ctx context.Context, scheduleID string) error {
	return manager.bootout(ctx, scheduleID)
}

func (manager Manager) bootstrap(ctx context.Context, scheduleID string) error {
	return manager.run(ctx, "bootstrap", "gui/"+manager.UID, manager.PlistPath(scheduleID))
}

func (manager Manager) bootout(ctx context.Context, scheduleID string) error {
	err := manager.run(ctx, "bootout", "gui/"+manager.UID+"/"+Label(scheduleID))
	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("unload LaunchAgent %s: %w", Label(scheduleID), err)
	}
	return nil
}

func (manager Manager) Inspect(ctx context.Context, schedule domain.Schedule) Status {
	path := manager.PlistPath(schedule.ID)
	status := Status{Label: Label(schedule.ID), PlistPath: path, Installed: fileExists(path), Enabled: schedule.Enabled}
	if !status.Installed {
		return status
	}
	status.Loaded = manager.run(ctx, "print", "gui/"+manager.UID+"/"+status.Label) == nil
	return status
}

func (manager Manager) run(ctx context.Context, args ...string) error {
	command := exec.CommandContext(ctx, manager.LaunchctlPath, args...)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl %s: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return nil
}

type plist struct {
	XMLName               xml.Name           `xml:"plist"`
	Version               string             `xml:"version,attr"`
	Label                 string             `xml:"dict>key"`
	ProgramArguments      []string           `xml:"-"`
	StartInterval         *int               `xml:"-"`
	StartCalendarInterval []calendarInterval `xml:"-"`
	StandardOutPath       string             `xml:"-"`
	StandardErrorPath     string             `xml:"-"`
	Environment           map[string]string  `xml:"-"`
}

type calendarInterval struct {
	Hour   int `xml:"Hour"`
	Minute int `xml:"Minute"`
	Second int `xml:"Second,omitempty"`
}

func (value plist) MarshalXML(encoder *xml.Encoder, start xml.StartElement) error {
	start.Name.Local = "plist"
	start.Attr = []xml.Attr{{Name: xml.Name{Local: "version"}, Value: value.Version}}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "dict"}}); err != nil {
		return err
	}
	if err := encodeKeyString(encoder, "Label", value.Label); err != nil {
		return err
	}
	if err := encodeKeyStringArray(encoder, "ProgramArguments", value.ProgramArguments); err != nil {
		return err
	}
	if value.StartInterval != nil {
		if err := encodeKeyInteger(encoder, "StartInterval", *value.StartInterval); err != nil {
			return err
		}
	}
	if len(value.StartCalendarInterval) > 0 {
		if err := encodeKeyCalendarArray(encoder, "StartCalendarInterval", value.StartCalendarInterval); err != nil {
			return err
		}
	}
	if value.StandardOutPath != "" {
		if err := encodeKeyString(encoder, "StandardOutPath", value.StandardOutPath); err != nil {
			return err
		}
	}
	if value.StandardErrorPath != "" {
		if err := encodeKeyString(encoder, "StandardErrorPath", value.StandardErrorPath); err != nil {
			return err
		}
	}
	if len(value.Environment) > 0 {
		if err := encodeKeyStringDict(encoder, "EnvironmentVariables", value.Environment); err != nil {
			return err
		}
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "dict"}}); err != nil {
		return err
	}
	return encoder.EncodeToken(xml.EndElement{Name: start.Name})
}

func encodeKey(encoder *xml.Encoder, value string) error {
	start := xml.StartElement{Name: xml.Name{Local: "key"}}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.CharData(value)); err != nil {
		return err
	}
	return encoder.EncodeToken(xml.EndElement{Name: start.Name})
}

func encodeKeyString(encoder *xml.Encoder, key, value string) error {
	if err := encodeKey(encoder, key); err != nil {
		return err
	}
	return encoder.EncodeElement(value, xml.StartElement{Name: xml.Name{Local: "string"}})
}

func encodeKeyStringArray(encoder *xml.Encoder, key string, values []string) error {
	if err := encodeKey(encoder, key); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "array"}}); err != nil {
		return err
	}
	for _, value := range values {
		if err := encoder.EncodeElement(value, xml.StartElement{Name: xml.Name{Local: "string"}}); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "array"}})
}

func encodeKeyInteger(encoder *xml.Encoder, key string, value int) error {
	if err := encodeKey(encoder, key); err != nil {
		return err
	}
	return encoder.EncodeElement(value, xml.StartElement{Name: xml.Name{Local: "integer"}})
}

func encodeKeyStringDict(encoder *xml.Encoder, key string, values map[string]string) error {
	if err := encodeKey(encoder, key); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "dict"}}); err != nil {
		return err
	}
	keys := make([]string, 0, len(values))
	for valueKey := range values {
		keys = append(keys, valueKey)
	}
	sort.Strings(keys)
	for _, valueKey := range keys {
		if err := encodeKeyString(encoder, valueKey, values[valueKey]); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "dict"}})
}

func encodeKeyCalendarArray(encoder *xml.Encoder, key string, values []calendarInterval) error {
	if err := encodeKey(encoder, key); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "array"}}); err != nil {
		return err
	}
	for _, value := range values {
		if err := encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "dict"}}); err != nil {
			return err
		}
		if err := encodeKeyInteger(encoder, "Hour", value.Hour); err != nil {
			return err
		}
		if err := encodeKeyInteger(encoder, "Minute", value.Minute); err != nil {
			return err
		}
		if value.Second != 0 {
			if err := encodeKeyInteger(encoder, "Second", value.Second); err != nil {
				return err
			}
		}
		if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "dict"}}); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "array"}})
}

func parseClock(value string) (int, int, int, error) {
	value = strings.TrimSpace(value)
	parsed, err := time.Parse("15:04", value)
	if err == nil {
		return parsed.Hour(), parsed.Minute(), 0, nil
	}
	parsed, err = time.Parse("15:04:05", value)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid daily time %q: expected HH:MM or HH:MM:SS", value)
	}
	return parsed.Hour(), parsed.Minute(), parsed.Second(), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isNotFoundError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "could not find service") || strings.Contains(message, "service not found") || strings.Contains(message, "no such process") || strings.Contains(message, "not found")
}

func atomicWrite(path string, contents []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".open-agent-clock-plist-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
