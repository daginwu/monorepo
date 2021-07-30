package timetrace

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/daginwu/monorepo/libs/epaxos/src/cycles"
)

/**
 * This class implements a circular buffer of entries, each of which
 * consists of a fine-grain timestamp, a short descriptive string, and
 * a few additional values. It's typically used to record times at
 * various points in an operation, in order to find performance bottlenecks.
 * It can record a trace relatively efficiently (< 10ns as of 6/2016),
 * and then either return the trace either as a string or print it to
 * the system log.
 *
 * This class is thread-safe. By default, trace information is recorded
 * separately for each thread in order to avoid synchronization and cache
 * consistency overheads; the thread-local traces are merged by methods
 * such as printToLog, so the existence of multiple trace buffers is
 * normally invisible.
 *
 * The TimeTrace class should never be constructed; it offers only
 * static methods.
 *
 * If you want to use a single trace buffer rather than per-thread
 * buffers, see the subclass TimeTrace::Buffer below.
 */
type TimeTrace struct {
	sync.Mutex

	// Holds pointers to all of the thread-private TimeTrace objects created
	// so far. Entries never get deleted from this object.
	threadBuffers []*Buffer

	frozen bool
}

type Context struct {
	// Points to a private per-thread TimeTrace::Buffer object; NULL means
	// no such object has been created yet for the current thread.
	ThreadBuffer *Buffer
}

var globalTrace *TimeTrace

func Init() {
	globalTrace = &TimeTrace{}
	globalTrace.threadBuffers = []*Buffer{}
	cycles.Init()
}

/**
 * This structure holds one entry in a TimeTrace::Buffer.
 */
type Event struct {
	// Time when a particular event occurred.
	timestamp uint64

	// Format string describing the event.
	// nil means that this entry is unused.
	format *string

	// Number of valid args in the args array
	nargs int32

	// Arguments that may be referenced by format
	// when printing out this event.
	args [10]int64
}

func NewContext() *Context {
	return globalTrace.NewContext()
}

/**
 * Return a thread-local buffer that can be used to record events from the
 * calling thread.
 */
func (t *TimeTrace) NewContext() *Context {
	b := NewBuffer()

	t.Lock()
	t.threadBuffers = append(t.threadBuffers, b)
	t.Unlock()
	c := Context{b}
	return &c
}

func Record(context *Context, format *string, nargs int32, arg1 int64, arg2 int64,
	arg3 int64, arg4 int64) {
	globalTrace.Record(context, format, nargs, arg1, arg2, arg3, arg4)
}

func Record0(context *Context, format *string) {
	globalTrace.Record(context, format, 0, 0, 0, 0, 0)
}

func Record1(context *Context, format *string, arg1 int64) {
	globalTrace.Record(context, format, 1, arg1, 0, 0, 0)
}

func Record2(context *Context, format *string, arg1 int64, arg2 int64) {
	globalTrace.Record(context, format, 2, arg1, arg2, 0, 0)
}

func Record3(context *Context, format *string, arg1 int64, arg2 int64,
	arg3 int64) {
	globalTrace.Record(context, format, 3, arg1, arg2, arg3, 0)
}

func Record4(context *Context, format *string, arg1 int64, arg2 int64,
	arg3 int64, arg4 int64) {
	globalTrace.Record(context, format, 4, arg1, arg2, arg3, arg4)
}

/**
 * Record an event in a thread-local buffer, creating a new buffer
 * if this is the first record for this thread.
 *
 * \param timestamp
 *      Identifies the time at which the event occurred.
 * \param format
 *      A format string for snprintf that will be used, along with
 *      arg0..arg3, to generate a human-readable message describing what
 *      happened, when the time trace is printed. The message is generated
 *      by calling snprintf as follows:
 *      snprintf(buffer, size, format, arg0, arg1, arg2, arg3)
 *      where format and arg0..arg3 are the corresponding arguments to this
 *      method. This pointer is stored in the time trace, so the caller must
 *      ensure that its contents will not change over its lifetime in the
 *      trace.
 * \param args
 *      Arguments to use when printing a message about this event.
 */
func (t *TimeTrace) Record(context *Context, format *string, nargs int32, arg1 int64, arg2 int64,
	arg3 int64, arg4 int64) {
	if !t.frozen {
		t.record(cycles.Rdtsc(), context, format, nargs, arg1, arg2, arg3, arg4)
	}
}

func (t *TimeTrace) record(timestamp uint64, context *Context, format *string, nargs int32, arg1 int64, arg2 int64,
	arg3 int64, arg4 int64) {
	if !t.frozen {
		context.ThreadBuffer.record(timestamp, format, nargs, arg1, arg2, arg3, arg4, 0, 0, 0, 0, 0)
	}
}

func GetTrace() string {
	return globalTrace.GetTrace()
}

/**
 * Return a string containing all of the trace records from all of the
 * thread-local buffers.
 * NOT safe with logging timetraces
 */
func (t *TimeTrace) GetTrace() string {
	var s string
	printInternal(t.threadBuffers, &s, nil)
	return s
}

func (t *TimeTrace) LogTrace(f *os.File) {
	printInternal(t.threadBuffers, nil, f)
}

/**
 * Discards all records in all of the thread-local buffers. Intended
 * primarily for unit testing.
 */
func (t *TimeTrace) Reset() {
	t.Lock()
	for _, b := range t.threadBuffers {
		b.Reset()
	}
	t.Unlock()
}

func Reset() {
	globalTrace.Reset()
}

const (
	// Determines the number of events we can retain as an exponent of 2
	BUFFER_SIZE_EXP = 14

	// Total number of events that we can retain any given time.
	BUFFER_SIZE = 1 << BUFFER_SIZE_EXP

	// Bit mask used to implement a circular event buffer
	BUFFER_MASK = BUFFER_SIZE - 1
)

/**
 * Represents a sequence of events, typically consisting of all those
 * generated by one thread.  Has a fixed capacity, so slots are re-used
 * on a circular basis.  This class is not thread-safe.
 */
type Buffer struct {
	// Index within events of the slot to use for the next call to the
	// record method.
	nextIndex int

	// Holds information from the most recent calls to the record method.
	events [BUFFER_SIZE]Event
}

/**
 * Construct a TimeTrace::Buffer.
 */
func NewBuffer() *Buffer {
	var b Buffer
	b.nextIndex = 0

	// Mark all of the events invalid.
	for i := 0; i < BUFFER_SIZE; i++ {
		b.events[i].format = nil
	}

	return &b
}

/**
 * Return a string containing a printout of the records in the buffer.
 */
func (b *Buffer) GetTrace() string {
	buffers := []*Buffer{b}
	var s string
	printInternal(buffers, &s, nil)
	return s
}

func (b *Buffer) Record0(format *string) {
	b.Record(format, 0, 0, 0, 0, 0)
}

func (b *Buffer) Record1(format *string, arg1 int64) {
	b.Record(format, 1, arg1, 0, 0, 0)
}

func (b *Buffer) Record2(format *string, arg1 int64, arg2 int64) {
	b.Record(format, 2, arg1, arg2, 0, 0)
}

func (b *Buffer) Record3(format *string, arg1 int64, arg2 int64,
	arg3 int64) {
	b.Record(format, 3, arg1, arg2, arg3, 0)
}

func (b *Buffer) Record9(format *string, arg1 int64, arg2 int64,
	arg3 int64, arg4 int64, arg5 int64, arg6 int64, arg7 int64, arg8 int64, arg9 int64) {
	b.record(cycles.Rdtsc(), format, 9, arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9)
}

func (b *Buffer) Record4(format *string, arg1 int64, arg2 int64,
	arg3 int64, arg4 int64) {
	b.Record(format, 4, arg1, arg2, arg3, arg4)
}

/**
 * Record an event in the buffer.
 *
 * \param format
 *      A format string for snprintf that will be used, along with
 *      arg0..arg3, to generate a human-readable message describing what
 *      happened, when the time trace is printed. The message is generated
 *      by calling snprintf as follows:
 *      snprintf(buffer, size, format, arg0, arg1, arg2, arg3)
 *      where format and arg0..arg3 are the corresponding arguments to this
 *      method. This pointer is stored in the buffer, so the caller must
 *      ensure that its contents will not change over its lifetime in the
 *      trace.
 * \param args
 *      Arguments to use when printing a message about this event.
 */
func (b *Buffer) record(timestamp uint64, format *string, nargs int32, arg1 int64, arg2 int64,
	arg3 int64, arg4 int64, arg5 int64, arg6 int64, arg7 int64, arg8 int64, arg9 int64) {
	event := &b.events[b.nextIndex]
	event.timestamp = timestamp
	event.format = format
	event.nargs = nargs
	event.args[0] = arg1
	event.args[1] = arg2
	event.args[2] = arg3
	event.args[3] = arg4
	event.args[4] = arg5
	event.args[5] = arg6
	event.args[6] = arg7
	event.args[7] = arg8
	event.args[8] = arg9
	b.nextIndex = (b.nextIndex + 1) & BUFFER_MASK
}

func (b *Buffer) Record(format *string, nargs int32, arg1 int64, arg2 int64,
	arg3 int64, arg4 int64) {
	b.record(cycles.Rdtsc(), format, nargs, arg1, arg2, arg3, arg4, 0, 0, 0, 0, 0)
}

/**
 * Discard any existing trace records.
 */
func (b *Buffer) Reset() {
	for i := 0; i < BUFFER_SIZE; i++ {
		b.events[i].format = nil
	}
	b.nextIndex = 0
}

func (b *Buffer) DumpTrace(fname string) {
	f, err := os.Create(fname)
	if err != nil {
		log.Println("Couldn't log time trace in the background:", err)
		return
	}

	printInternal([]*Buffer{b}, nil, f)
}

/**
 * This private method does most of the work for both printToLog and
 * getTrace.
 *
 * \param buffers
 *      Contains one or more TimeTrace::Buffers, whose contents will be merged
 *      in the resulting output. Note: some of the buffers may extend
 *      farther back in time than others. The output will cover only the
 *      time period covered by *all* of the traces, ignoring older entries
 *      from some traces.
 * \param s
 *      If non-NULL, refers to a string that will hold a printout of the
 *      time trace. If NULL, the trace will be printed on the system log.
 */
func printInternal(buffers []*Buffer, s *string, f *os.File) {
	printedAnything := false

	// Holds the index of the next event to consider from each trace.
	current := make([]int, len(buffers))

	// Find the first (oldest) event in each trace. This will be events[0]
	// if we never completely filled the buffer, otherwise events[nextIndex+1].
	// This means we don't print the entry at nextIndex; this is convenient
	// because it simplifies boundary conditions in the code below.
	for i, buffer := range buffers {
		index := (buffer.nextIndex + 1) % BUFFER_SIZE
		if buffer.events[index].format != nil {
			current[i] = index
		} else {
			current[i] = 0
		}
	}

	// Decide on the time of the first event to be included in the output.
	// This is most recent of the oldest times in all the traces (an empty
	// trace has an "oldest time" of 0). The idea here is to make sure
	// that there's no missing data in what we print (if trace A goes back
	// farther than trace B, skip the older events in trace A, since there
	// might have been related events that were once in trace B but have since
	// been overwritten).
	var startTime uint64 = 0
	for i := 0; i < len(buffers); i++ {
		event := buffers[i].events[current[i]]
		if event.format != nil && event.timestamp > startTime {
			startTime = event.timestamp
		}
	}
	log.Printf("Starting TSC %d, cyclesPerSec %.0f\n", startTime,
		cycles.PerSecond())

	// Skip all events before the starting time.
	for i := 0; i < len(buffers); i++ {
		buffer := buffers[i]
		for buffer.events[current[i]].format != nil &&
			buffer.events[current[i]].timestamp < startTime &&
			current[i] != buffer.nextIndex {
			current[i] = (current[i] + 1) % BUFFER_SIZE
		}
	}

	// Each iteration through this loop processes one event (the one with
	// the earliest timestamp).
	prevTime := 0.0

	var stringBuilder strings.Builder
	for {
		var buffer *Buffer
		var event *Event

		// Check all the traces to find the earliest available event.
		currentBuffer := -1
		var earliestTime uint64 = ^uint64(0)
		for i, buffer := range buffers {
			event := buffer.events[current[i]]
			if current[i] != buffer.nextIndex && event.format != nil && event.timestamp < earliestTime {
				currentBuffer = i
				earliestTime = event.timestamp
			}
		}
		if currentBuffer < 0 {
			// None of the traces have any more events to process.
			break
		}
		printedAnything = true
		buffer = buffers[currentBuffer]
		event = &buffer.events[current[currentBuffer]]
		current[currentBuffer] = (current[currentBuffer] + 1) % BUFFER_SIZE

		ns := cycles.ToNanoseconds(event.timestamp - startTime)

		args := make([]interface{}, event.nargs)
		for i := int32(0); i < event.nargs; i++ {
			args[i] = event.args[i]
		}

		stringBuilder.WriteString(fmt.Sprintf("T%d %8.1f ns (+%6.1f ns): ",
			currentBuffer+1, ns, ns-prevTime))
		stringBuilder.WriteString(fmt.Sprintf(*event.format, args...))
		stringBuilder.WriteString("\n")

		prevTime = ns
	}

	if !printedAnything {
		stringBuilder.WriteString("No time trace events to print\n")
	}

	if s != nil {
		*s = stringBuilder.String()
	} else if f != nil {
		f.WriteString(stringBuilder.String())
	}
}
