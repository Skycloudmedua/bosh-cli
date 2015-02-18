package fakes

import (
	bmui "github.com/cloudfoundry/bosh-micro-cli/ui"
)

type FakeStage struct {
	PerformCalls []PerformCall
	SubStages    []*FakeStage
}

type PerformCall struct {
	Name      string
	Error     error
	SkipError error
	Stage     *FakeStage
}

func NewFakeStage() *FakeStage {
	return &FakeStage{}
}

func (s *FakeStage) Perform(name string, closure func() error) error {
	err := closure()

	call := PerformCall{Name: name, Error: err}

	if err != nil {
		if skipErr, isSkipError := err.(bmui.SkipStageError); isSkipError {
			call.SkipError = skipErr
			err = nil
		}
	}

	// lazily instantiate to make matching sub-stages easier
	if s.PerformCalls == nil {
		s.PerformCalls = []PerformCall{}
	}
	s.PerformCalls = append(s.PerformCalls, call)

	return err
}

func (s *FakeStage) PerformComplex(name string, closure func(bmui.Stage) error) error {
	subStage := NewFakeStage()

	// lazily instantiate to make matching simple stages easier
	if s.SubStages == nil {
		s.SubStages = []*FakeStage{}
	}
	s.SubStages = append(s.SubStages, subStage)

	err := closure(subStage)

	call := PerformCall{Name: name, Error: err, Stage: subStage}

	if err != nil {
		if skipErr, isSkipError := err.(bmui.SkipStageError); isSkipError {
			call.SkipError = skipErr
			err = nil
		}
	}

	// lazily instantiate to make matching sub-stages easier
	if s.PerformCalls == nil {
		s.PerformCalls = []PerformCall{}
	}
	s.PerformCalls = append(s.PerformCalls, call)

	return err
}