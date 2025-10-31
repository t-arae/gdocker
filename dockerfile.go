package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

var (
	// global separater for parsing --inst options
	ARG_SEPARATER     = ":"
	OLD_ARG_SEPARATER string
	ONCE              = false
)

// Dockerfile
type Dockerfile struct {
	stages    []Stage
	numStages int
	usePreset bool
}

func NewDockerfile(preset bool) Dockerfile {
	d := Dockerfile{}
	if preset {
		d.usePreset = true
		d.AddStage(Stage{})
		d.LastStage().AddInstruction(&COMMENT{"syntax=docker/dockerfile:1"})
		d.LastStage().AddInstruction(&COMMENT{"This Dockerfile is created using gdocker."})
	}
	return d
}

func (d *Dockerfile) AddStage(stage Stage) {
	d.stages = append(d.stages, stage)
	d.numStages += 1
}

func (d *Dockerfile) LastStage() *Stage {
	return &(d.stages[d.numStages-1])
}

func (d *Dockerfile) AddInstruction(inst InstructionBuilder) {
	if _, isFrom := inst.(*FROM_INST); isFrom {
		d.AddStage(Stage{})
		d.LastStage().containsFrom = true
	}
	if d.numStages == 0 {
		if _, iscomment := inst.(*COMMENT); iscomment {
			d.AddStage(Stage{})
		} else {
			slog.Error("first instruction must be FROM")
			os.Exit(1)
		}
	}
	if !d.LastStage().containsFrom {
		switch v := inst.(type) {
		case *FROM_INST:
		case *COMMENT:
		case *BLANK:
		//case *ARG:
		default:
			slog.Error(fmt.Sprintf("%T is disallowed before FROM", v))
			os.Exit(1)
		}
	}
	d.LastStage().AddInstruction(inst)
}

func (d *Dockerfile) AddInstructionAt(at int, inst InstructionBuilder) {
	d.stages[at].AddInstruction(inst)
}

func (d *Dockerfile) addFooter() {
	if d.usePreset {
		presetFooter := []InstructionBuilder{
			&BLANK{},
			&LABEL_INST{[]string{fmt.Sprintf("com.gdocker.version=v%s", APP_VERSION)}},
			&BLANK{},
			&WORKDIR_INST{"/data"},
			&ENTRYPOINT_INST{"exec", []string{"/usr/local/bin/entrypoint.sh"}, "!"},
		}
		for _, inst := range presetFooter {
			d.LastStage().AddInstruction(inst)
		}
	}
}

func (d *Dockerfile) BuildAll() {
	d.addFooter()
	var buf bytes.Buffer
	for _, stage := range d.stages {
		stage.appendBuffer(&buf)
	}
	buf.WriteTo(os.Stdout)
}

func (d *Dockerfile) WriteTo(file string, box bool) {
	d.addFooter()

	var err error
	var buf bytes.Buffer
	for _, stage := range d.stages {
		stage.appendBuffer(&buf)
	}

	var w io.Writer
	if file == "stdout" {
		w = os.Stdout
	} else {
		if box {
			w = &BoxedWriter{Title: anonymizeWd(file, true), Out: os.Stdout}
		} else {
			var f *os.File
			f, err = os.Create(file)
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
			defer f.Close()
			w = f
		}
	}
	_, err = w.Write(buf.Bytes())
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

// Stage
type Stage struct {
	Instructions []InstructionBuilder
	containsFrom bool
}

func (s *Stage) AddInstruction(inst InstructionBuilder) {
	s.Instructions = append(s.Instructions, inst)
}

func (s Stage) appendBuffer(buf *bytes.Buffer) {
	s.Merge()
	s.MergeApt()
	if len(s.Instructions) > 0 {
		for _, inst := range s.Instructions {
			buf.WriteString(inst.Build())
			buf.WriteString("\n")
		}
	}
}

func (s *Stage) Merge() {
	var new []InstructionBuilder
	merged := RUN_INST{}
	for _, inst := range s.Instructions {
		r, is_run := inst.(*RUN_INST)
		// append
		if !is_run || r.operator == "!" {
			if merged.form == "" {
				// temporal inst only
				new = append(new, inst)
				continue
			} else {
				// both merged and temporal inst
				toadd := RUN_INST{}
				toadd.form = merged.form
				toadd.command = append(toadd.command, merged.command...)
				toadd.operator = "!"
				new = append(new, &toadd)
				new = append(new, inst)
				merged = RUN_INST{}
				continue
			}
		}

		if merged.form == "" {
			merged.form = r.form
			merged.command = r.command
			merged.operator = r.operator
		} else {
			merged.command = append(merged.command, "\\", merged.operator)
			merged.command = append(merged.command, r.command...)
			merged.operator = r.operator
		}
	}
	s.Instructions = new
}

func (s *Stage) MergeApt() {
	merged := APT_INSTALL{}
	for _, inst := range s.Instructions {
		a, is_apt := inst.(*APT_INSTALL)
		// append
		if is_apt {
			if merged.form == "shell" {
				merged.command = append(merged.command, a.command...)
			} else {
				merged.form = "shell"
				merged.command = a.command
				merged.operator = "!"
			}
		}
	}
	if merged.form == "" {
		return
	}
	unuse := true
	var new []InstructionBuilder
	for _, inst := range s.Instructions {
		_, is_apt := inst.(*APT_INSTALL)
		if is_apt {
			if unuse {
				new = append(new, &merged)
				unuse = false
			} else {
				continue
			}
		} else {
			new = append(new, inst)
		}
	}
	s.Instructions = new
}

// InstructionBuilder
type InstructionBuilder interface {
	Build() string
	Parse(string)
	ParseShort(string)
}

// blank or comment line
type COMMENT struct {
	comment string
}

func (comment COMMENT) Build() string {
	args := []string{}
	if comment.comment != "" {
		args = append(args, []string{"#", comment.comment}...)
	}
	return strings.Join(args, " ")
}

func (comment *COMMENT) Parse(s string) {
	comment.comment = s
}

func (comment *COMMENT) ParseShort(s string) {
	comment.Parse(s)
}

type BLANK COMMENT

func (blank BLANK) Build() string { return "" }

func (blank *BLANK) Parse(s string) {}

func (blank *BLANK) ParseShort(s string) {}

// FROM
type FROM_INST struct {
	platform string
	image    string
	tag      string
	AS       string
}

func (from FROM_INST) Build() string {
	args := []string{"FROM"}
	if from.platform != "" {
		args = append(args, fmt.Sprintf("--platform=%s", from.platform))
	}
	if from.tag == "" {
		args = append(args, from.image)
	} else {
		args = append(args, fmt.Sprintf("%s:%s", from.image, from.tag))
	}
	if from.AS != "" {
		args = append(args, []string{"AS", from.AS}...)
	}
	return strings.Join(args, " ")
}

func (from *FROM_INST) Parse(s string) {
	args := strings.Split(s, ARG_SEPARATER)
	from.platform = args[0]
	from.image = args[1]
	from.tag = args[2]
	from.AS = args[3]
}

func (from *FROM_INST) ParseShort(s string) {
	args := strings.Split(s, ARG_SEPARATER)
	switch len(args) {
	case 1:
		from.image = args[0]
	case 2:
		from.image = args[0]
		from.tag = args[1]
	case 3:
		from.image = args[0]
		from.tag = args[1]
		from.AS = args[2]
	default:
		slog.Error(fmt.Sprintf("invalid from string '%s'", s))
		os.Exit(1)
	}
}

// common instruction build routine of RUN, CMD and ENTRYPOINT
func commonBuild(inst, form string, command []string) string {
	args := []string{inst}

	// check form
	if form != "shell" && form != "exec" {
		slog.Error(fmt.Sprintf("form must be either shell or exec, '%s'", form))
		os.Exit(1)
	}

	if form == "shell" {
		args = append(args, command...)
		for i, v := range args {
			if v == `\` {
				args[i] = `\
	`
			}
		}
	} else {
		out, _ := json.Marshal(command)
		args = append(args, string(out))
	}
	return strings.Join(args, " ")
}

// RUN
type RUN_INST struct {
	form     string
	command  []string
	operator string
}

func (run RUN_INST) Build() string { return commonBuild("RUN", run.form, run.command) }

func (run *RUN_INST) Parse(s string) {
	form, cut_form, ok := strings.Cut(s, ARG_SEPARATER)
	if !ok {
		slog.Error(fmt.Sprintf("no form found. '%s'", s))
		os.Exit(1)
	}
	sep, rest := separatorCheck(cut_form)
	com, op := operatorCheck(rest)
	run.form = form
	run.command = strings.Split(com, sep)
	run.operator = op
}

func (run *RUN_INST) ParseShort(s string) {
	sep, rest := separatorCheck(s)
	com, op := operatorCheck(rest)
	run.form = "shell"
	run.command = strings.Split(com, sep)
	run.operator = op
}

// apt install
type APT_INSTALL RUN_INST

func (apt APT_INSTALL) Build() string {
	command := []string{"apt-get", "update", "&&", "apt-get", "install", "-y", "--no-install-recommends", "\\"}
	command = append(command, apt.command...)
	command = append(command, []string{"&&", "apt-get", "clean", "-y", "\\"}...)
	command = append(command, []string{"&&", "rm", "-rf", "/var/lib/apt/list/*"}...)
	return commonBuild("RUN", apt.form, command)
}

func (apt *APT_INSTALL) Parse(s string) {
	apt.form = "shell"
	for _, pkg := range strings.Split(s, ",") {
		apt.command = append(apt.command, pkg, "\\")
	}
}

func (apt *APT_INSTALL) ParseShort(s string) {
	apt.Parse(s)
}

// ENTRYPOINT
type ENTRYPOINT_INST RUN_INST

func (ep ENTRYPOINT_INST) Build() string {
	return commonBuild("ENTRYPOINT", ep.form, ep.command)
}

func (ep *ENTRYPOINT_INST) Parse(s string) {
	form, cut_form, ok := strings.Cut(s, ARG_SEPARATER)
	if !ok {
		slog.Error(fmt.Sprintf("no form found. '%s'", s))
		os.Exit(1)
	}
	ep.form = form
	sep, rest := separatorCheck(cut_form)
	ep.command = strings.Split(rest, sep)
}

func (ep *ENTRYPOINT_INST) ParseShort(s string) {
	sep, rest := separatorCheck(s)
	ep.form = "exec"
	ep.command = strings.Split(rest, sep)
}

// COPY
type COPY_INST struct {
	form    string
	options []string
	src     []string
	dest    string
}

func (copy COPY_INST) Build() string {
	args := []string{"COPY"}
	if len(copy.options) != 0 {
		args = append(args, copy.options...)
	}

	if copy.form == "shell" {
		args = append(args, append(copy.src, copy.dest)...)
	} else {
		out, _ := json.Marshal(append(copy.src, copy.dest))
		args = append(args, string(out))
	}
	return strings.Join(args, " ")
}

func (cp *COPY_INST) Parse(s string) {
	form, cut_form, ok := strings.Cut(s, ARG_SEPARATER)
	if !ok {
		slog.Error(fmt.Sprintf("no form found. '%s'", s))
		os.Exit(1)
	}
	cp.form = form

	sep, cut_sep := separatorCheck(cut_form)

	option, cut_option, ok := strings.Cut(cut_sep, ARG_SEPARATER)
	if !ok {
		slog.Error(fmt.Sprintf("no option found. '%s'", cut_sep))
		os.Exit(1)
	}
	cp.options = strings.Split(option, sep)

	src, dest, ok := strings.Cut(cut_option, ARG_SEPARATER)
	if !ok {
		slog.Error(fmt.Sprintf("no src found. '%s'", cut_option))
		os.Exit(1)
	}
	cp.src = strings.Split(src, sep)
	cp.dest = dest
}

func (cp *COPY_INST) ParseShort(s string) {
	sep, cut_sep := separatorCheck(s)

	src, cut_src, found := strings.Cut(cut_sep, ARG_SEPARATER)
	if !found {
		slog.Error(fmt.Sprintf("no src found. '%s'", cut_sep))
		os.Exit(1)
	}

	dest, option, found := strings.Cut(cut_src, ARG_SEPARATER)
	if !found {
		dest = cut_src
	}

	cp.form = "shell"
	cp.options = strings.Split(option, sep)
	cp.src = strings.Split(src, sep)
	cp.dest = dest
}

// WORKDIR
type WORKDIR_INST struct {
	workingdirectory string
}

func (wd WORKDIR_INST) Build() string {
	args := []string{"WORKDIR", wd.workingdirectory}
	return strings.Join(args, " ")
}

func (wd *WORKDIR_INST) Parse(s string) {
	wd.workingdirectory = s
}

func (wd *WORKDIR_INST) ParseShort(s string) {
	wd.Parse(s)
}

// ENV
type ENV_INST struct {
	keys   []string
	values []string
}

func (env ENV_INST) Build() string {
	args := []string{"ENV"}
	if len(env.keys) != len(env.values) {
		slog.Error("different length in keys and values of ENV")
		os.Exit(1)
	}
	for i, k := range env.keys {
		args = append(args, fmt.Sprintf("%s=%s", k, env.values[i]))
	}

	return strings.Join(args, " ")
}

func (env *ENV_INST) Parse(s string) {
	kv_pairs := strings.Split(s, ARG_SEPARATER)
	if len(kv_pairs) == 0 {
		slog.Error("invalid ENV")
		os.Exit(1)
	}
	for _, pair := range kv_pairs {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			slog.Error(fmt.Sprintf("invalid key-value pair '%s'", pair))
			os.Exit(1)
		}
		env.keys = append(env.keys, key)
		env.values = append(env.values, value)
	}
}

func (env *ENV_INST) ParseShort(s string) {
	env.Parse(s)
}

// VOLUME
type VOLUMNE_INST struct {
	volumes []string
}

func (volume VOLUMNE_INST) Build() string {
	args := []string{"VOLUME"}

	out, _ := json.Marshal(volume.volumes)
	args = append(args, string(out))

	return strings.Join(args, " ")
}

func (volume *VOLUMNE_INST) Parse(s string) {
	volumes := strings.Split(s, ARG_SEPARATER)
	if len(volumes) == 0 {
		slog.Error("invalid VOLUME")
		os.Exit(1)
	}
	volume.volumes = volumes
}

func (volume *VOLUMNE_INST) ParseShort(s string) {
	volume.Parse(s)
}

// LABEL
type LABEL_INST struct {
	labels []string
}

func (label LABEL_INST) Build() string {
	args := []string{"LABEL"}

	args = append(args, label.labels...)

	return strings.Join(args, " ")
}

func (label *LABEL_INST) Parse(s string) {
	kv_pairs := strings.Split(s, ARG_SEPARATER)
	if len(kv_pairs) == 0 {
		slog.Error("invalid LABEL")
		os.Exit(1)
	}
	for _, pair := range kv_pairs {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			slog.Error(fmt.Sprintf("invalid key-value pair '%s'", pair))
			os.Exit(1)
		}
		label.labels = append(label.labels, fmt.Sprintf("%s=%s", key, value))
	}
}

func (label *LABEL_INST) ParseShort(s string) {
	label.Parse(s)
}

// Utility functions
func separatorCheck(s string) (sep string, rest string) {
	sep, rest, found := strings.Cut(s, ARG_SEPARATER)
	if !found {
		slog.Error(fmt.Sprintf("no separator found. '%s'", s))
		os.Exit(1)
	}
	// default separator
	if sep == "" {
		sep = ","
	}
	return sep, rest
}

func operatorCheck(s string) (string, op string) {
	pre, op, found := strings.Cut(s, ARG_SEPARATER)
	if !found {
		op = "&&"
	}
	return pre, op
}
