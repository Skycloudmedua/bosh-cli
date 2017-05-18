package task

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	boshui "github.com/cloudfoundry/bosh-cli/ui"
)

type ReporterImpl struct {
	ui          boshui.UI
	isForEvents bool

	events          map[int][]*Event
	eventMarkers    []eventMarker
	lastGlobalEvent *Event

	outputRest string
	sync.Mutex
}

type eventMarker struct {
	TaskID int
	Type   int
}

const (
	taskStarted = iota
	taskOutput  = iota
	taskEnded   = iota
)

func NewReporter(ui boshui.UI, isForEvents bool) *ReporterImpl {
	return &ReporterImpl{ui: ui, isForEvents: isForEvents, events: map[int][]*Event{}, eventMarkers: []eventMarker{}}
}

func (r *ReporterImpl) TaskStarted(id int) {
	r.Lock()
	defer r.Unlock()

	if r.lastEventWasTaskStarted() {
		r.ui.EndLinef("")
	}

	r.eventMarkers = append(r.eventMarkers, eventMarker{TaskID: id, Type: taskStarted})
	r.events[id] = []*Event{}

	r.ui.BeginLinef("Task %d", id)
}

func (r *ReporterImpl) TaskFinished(id int, state string) {
	r.Lock()
	defer r.Unlock()

	if len(r.events[id]) > 0 {
		start := r.events[id][0].TimeAsStr()
		end := r.lastEventForTask(id).TimeAsStr()
		duration := r.events[id][0].DurationAsStr(*r.lastEventForTask(id))
		r.ui.BeginLinef("\n\nTask %d Started  %s\nTask %d Finished %s\nTask %d Duration %s", id, start, id, end, id, duration)
	}

	if r.noOutputSinceTaskStarted(id) {
		r.ui.EndLinef(". %s", strings.Title(state))
	} else {
		r.ui.BeginLinef("\nTask %d %s\n", id, state)
	}

	r.eventMarkers = append(r.eventMarkers, eventMarker{TaskID: id, Type: taskEnded})
}

func (r *ReporterImpl) TaskOutputChunk(id int, chunk []byte) {
	r.Lock()
	defer r.Unlock()

	if r.noOutputSinceTaskStarted(id) {
		r.ui.BeginLinef("\n")
		if !r.isForEvents {
			r.ui.BeginLinef("\n")
		}
	}

	if r.isForEvents {
		r.outputRest += string(chunk)

		for {
			idx := strings.Index(r.outputRest, "\n")
			if idx == -1 {
				break
			}
			if len(r.outputRest[0:idx]) > 0 {
				r.showEvent(id, r.outputRest[0:idx])
			}
			r.outputRest = r.outputRest[idx+1:]
		}
	} else {
		r.showChunk(chunk)
	}

	r.eventMarkers = append(r.eventMarkers, eventMarker{TaskID: id, Type: taskOutput})
}

func (r *ReporterImpl) showEvent(id int, str string) {
	var event Event

	err := json.Unmarshal([]byte(str), &event)
	if err != nil {
		panic(fmt.Sprintf("unmarshal chunk '%s'", str))
	}

	for _, ev := range r.events[id] {
		if ev.IsSame(event) {
			event.StartEvent = ev
			break
		}
	}

	if r.lastGlobalEvent != nil && r.lastGlobalEvent.IsSame(event) {
		switch {
		case event.State == EventStateStarted:
			// does not make sense

		case event.State == EventStateFinished:
			r.ui.PrintBlock(fmt.Sprintf(" (%s)", event.DurationSinceStartAsStr()))

		case event.State == EventStateFailed:
			r.ui.PrintBlock(fmt.Sprintf(" (%s)", event.DurationSinceStartAsStr()))
			r.ui.PrintErrorBlock(fmt.Sprintf(
				"\n         L Task %d | Error: %s", id, event.Data.Error))
		}
	} else {
		if r.lastGlobalEvent != nil && event.IsWorthKeeping() {
			if event.Type == EventTypeDeprecation || event.Error != nil {
				// Some spacing around deprecations and errors
				r.ui.PrintBlock("\n")
			}
		}

		prefix := fmt.Sprintf("\n%s | Task %d | ", event.TimeAsHoursStr(), id)
		desc := event.Stage

		if len(event.Tags) > 0 {
			desc += " " + strings.Join(event.Tags, ", ")
		}

		switch {
		case event.Type == EventTypeDeprecation:
			r.ui.PrintBlock(prefix)
			r.ui.PrintErrorBlock(fmt.Sprintf("Deprecation: %s", event.Message))

		case event.Type == EventTypeWarning:
			r.ui.PrintBlock(prefix)
			r.ui.PrintErrorBlock(fmt.Sprintf("Warning: %s", event.Message))

		case event.State == EventStateStarted:
			r.ui.PrintBlock(prefix)
			r.ui.PrintBlock(fmt.Sprintf("%s: %s", desc, event.Task))

		case event.State == EventStateFinished:
			r.ui.PrintBlock(prefix)
			r.ui.PrintBlock(fmt.Sprintf("%s: %s (%s)",
				desc, event.Task, event.DurationSinceStartAsStr()))

		case event.State == EventStateFailed:
			r.ui.PrintBlock(prefix)
			r.ui.PrintBlock(fmt.Sprintf("%s: %s (%s)",
				desc, event.Task, event.DurationSinceStartAsStr()))
			r.ui.PrintErrorBlock(fmt.Sprintf(
				"\n         L Task %d | Error: %s", id, event.Data.Error))

		case event.Error != nil:
			r.ui.PrintBlock(prefix)
			r.ui.PrintErrorBlock(fmt.Sprintf("Error: %s", event.Error.Message))

		default:
			// Skip event
		}
	}

	if event.IsWorthKeeping() {
		r.events[id] = append(r.events[id], &event)
		r.lastGlobalEvent = &event
	}
}

func (r *ReporterImpl) showChunk(bytes []byte) {
	r.ui.PrintBlock(string(bytes))
}

func (r *ReporterImpl) lastEventForTask(id int) *Event {
	eventCount := len(r.events[id])
	if eventCount > 0 {
		return r.events[id][eventCount-1]
	}
	return nil
}

func (r *ReporterImpl) lastEventWasTaskStarted() bool {
	markerCount := len(r.eventMarkers)
	if markerCount == 0 {
		return false
	}
	lastMarker := r.eventMarkers[markerCount-1]
	return (lastMarker.Type == taskStarted)
}

func (r *ReporterImpl) noOutputSinceTaskStarted(id int) bool {
	markerCount := len(r.eventMarkers)
	if markerCount == 0 {
		return true
	}
	lastMarker := r.eventMarkers[markerCount-1]
	return (lastMarker.TaskID == id && lastMarker.Type == taskStarted)
}
