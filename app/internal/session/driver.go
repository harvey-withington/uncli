package session

import "uncli/internal/core"

// A CLI process is read by its adapter's parser and written to with its
// adapter's encoders, unless the adapter gives each process a driver
// (core.DriverAdapter, decision 0014: a protocol with requests both ways),
// which then does both and may have lines to write back. Every write to the
// CLI goes through encodeTurn and encodeControl, so the two paths can't
// diverge.

// feeder reads a process's output lines.
type feeder interface {
	feed(line []byte) (events []core.Event, replies [][]byte)
}

type parserFeeder struct{ p core.Parser }

func (f parserFeeder) feed(line []byte) ([]core.Event, [][]byte) {
	evs, err := f.p.Feed(line)
	if err != nil {
		evs = []core.Event{core.NewEvent(core.EvError, core.ErrorInfo{Message: err.Error()}, line)}
	}
	return evs, nil
}

type driverFeeder struct{ d core.Driver }

func (f driverFeeder) feed(line []byte) ([]core.Event, [][]byte) { return f.d.Feed(line) }

// feederFor sets up how the process about to start is read, and its driver
// if the adapter has one. Caller holds s.mu.
func (s *Session) feederFor(ad core.Adapter, spec core.LaunchSpec) feeder {
	s.driver = nil
	if da, ok := ad.(core.DriverAdapter); ok {
		if s.driver = da.NewDriver(spec); s.driver != nil {
			return driverFeeder{s.driver}
		}
	}
	return parserFeeder{ad.NewParser()}
}

// opening is what to write once the process has started.
func (s *Session) opening(ad core.Adapter) [][]byte {
	if s.driver != nil {
		return s.driver.Start()
	}
	if b, ok := ad.EncodeControl(core.Control{Kind: core.CtlInitialize}); ok {
		return [][]byte{b}
	}
	return nil
}

// encodeControl encodes a control for the running process. Caller holds s.mu.
func (s *Session) encodeControl(c core.Control) ([]byte, bool) {
	if s.driver != nil {
		return s.driver.Control(c)
	}
	return s.ad().EncodeControl(c)
}

// encodeTurn encodes a user turn; nil with no error: the driver sends it
// once it can. Caller holds s.mu.
func (s *Session) encodeTurn(t core.UserTurn) ([]byte, error) {
	if s.driver != nil {
		return s.driver.Turn(t)
	}
	return s.ad().EncodeTurn(t)
}

// writeBack writes a driver's replies to its process, if it still runs.
func (s *Session) writeBack(gen int, proc core.Proc, lines [][]byte) {
	if len(lines) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if gen != s.gen || s.proc != proc {
		return
	}
	for _, l := range lines {
		if _, err := proc.Stdin().Write(l); err != nil {
			return
		}
	}
}
