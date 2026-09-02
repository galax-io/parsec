//go:build integration

package text_test

import (
	"io"
	"strconv"
)

// synthLog streams a valid log of roughly the requested size without ever
// holding it in memory: a preamble, then a repeating block of realistic events
// with increasing timestamps, ending on a line boundary.
type synthLog struct {
	target   int64
	produced int64
	buf      []byte
	ts       int64
	isDone   bool
}

func newSynthLog(size int64) *synthLog {
	s := &synthLog{target: size, ts: 1788379356000}
	s.buf = append(s.buf, "ASSERTION\tAAEBAAICAAAAAAAAAPA/\n"...)
	s.buf = append(s.buf, "RUN\tio.galaxio.parsec.corpus.CorpusSimulation\tsynthetic\t1788379354534\t \t3.12.0\n"...)

	return s
}

func (s *synthLog) block() {
	t := strconv.FormatInt(s.ts, 10)
	end := strconv.FormatInt(s.ts+9, 10)
	s.ts += 10

	s.buf = append(s.buf, "USER\tsynthetic\tSTART\t"+t+"\n"...)
	s.buf = append(s.buf, "REQUEST\t\tGET /ok\t"+t+"\t"+end+"\tOK\t \n"...)
	s.buf = append(s.buf, "REQUEST\touter,inner\tGET /fail\t"+t+"\t"+end+"\tKO\tstatus.find.is(200), but actually found 500\n"...)
	s.buf = append(s.buf, "GROUP\touter,inner\t"+t+"\t"+end+"\t9\tKO\n"...)
	s.buf = append(s.buf, "GROUP\touter\t"+t+"\t"+end+"\t9\tKO\n"...)
	s.buf = append(s.buf, "ERROR\tsynthetic: crash \t"+t+"\n"...)
	s.buf = append(s.buf, "USER\tsynthetic\tEND\t"+end+"\n"...)
}

func (s *synthLog) Read(p []byte) (int, error) {
	for len(s.buf) < len(p) && !s.isDone {
		if s.produced+int64(len(s.buf)) >= s.target {
			s.isDone = true

			break
		}

		s.block()
	}

	if len(s.buf) == 0 {
		return 0, io.EOF
	}

	n := copy(p, s.buf)
	s.buf = s.buf[n:]
	s.produced += int64(n)

	return n, nil
}
