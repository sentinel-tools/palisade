package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
)

var (
	ExpectNumber   = &ProtocolError{"Expect Number"}
	ExpectNewLine  = &ProtocolError{"Expect Newline"}
	ExpectTypeChar = &ProtocolError{"Expect TypeChar"}

	InvalidNumArg   = errors.New("TooManyArg")
	InvalidBulkSize = errors.New("Invalid bulk size")
	LineTooLong     = errors.New("LineTooLong")

	ReadBufferInitSize = 1 << 10
	MaxNumArg          = 20
	MaxBulkSize        = 1 << 16
	MaxTelnetLine      = 1 << 10
	spaceSlice         = []byte{' '}
	emptyBulk          = [0]byte{}
)

const ()

type ProtocolError struct {
	message string
}

func (p *ProtocolError) Error() string {
	return p.message
}

type Command struct {
	argv [][]byte
}

func (c *Command) Get(index int) []byte {
	if index >= 0 && index < len(c.argv) {
		return c.argv[index]
	} else {
		return nil
	}
}
func (c *Command) ArgCount() int {
	return len(c.argv)
}

type RedisParser struct {
	reader        io.Reader
	buffer        []byte
	parsePosition int
	writeIndex    int
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func NewParser(reader io.Reader) *RedisParser {
	return &RedisParser{reader: reader, buffer: make([]byte, ReadBufferInitSize)}
}

// ensure that we have enough space for writing 'req' byte
func (r *RedisParser) requestSpace(req int) {
	ccap := cap(r.buffer)
	if r.writeIndex+req > ccap {
		newbuff := make([]byte, max(ccap*2, ccap+req+ReadBufferInitSize))
		copy(newbuff, r.buffer)
		r.buffer = newbuff
	}
}
func (r *RedisParser) readSome(min int) error {
	r.requestSpace(min)
	nr, err := io.ReadAtLeast(r.reader, r.buffer[r.writeIndex:], min)
	if err != nil {
		return err
	}
	r.writeIndex += nr
	return nil
}

// check for at least 'num' byte available in buffer to use, wait if need
func (r *RedisParser) requireNBytes(num int) error {
	a := r.writeIndex - r.parsePosition
	if a >= num {
		return nil
	}
	if err := r.readSome(num - a); err != nil {
		return err
	}
	return nil
}
func (r *RedisParser) readNumber() (int, error) {
	var neg bool = false
	err := r.requireNBytes(1)
	if err != nil {
		return 0, err
	}
	switch r.buffer[r.parsePosition] {
	case '-':
		neg = true
		r.parsePosition++
		break
	case '+':
		neg = false
		r.parsePosition++
		break
	}
	var num uint64 = 0
	var startpos int = r.parsePosition
OUTTER:
	for {
		for i := r.parsePosition; i < r.writeIndex; i++ {
			c := r.buffer[r.parsePosition]
			if c >= '0' && c <= '9' {
				num = num*10 + uint64(c-'0')
				r.parsePosition++
			} else {
				break OUTTER
			}
		}
		if r.parsePosition == r.writeIndex {
			if e := r.readSome(1); e != nil {
				return 0, e
			}
		}
	}
	if r.parsePosition == startpos {
		return 0, ExpectNumber
	}
	if neg {
		return -int(num), nil
	} else {
		return int(num), nil
	}

}
func (r *RedisParser) discardNewLine() error {
	if e := r.requireNBytes(2); e != nil {
		return e
	}
	if r.buffer[r.parsePosition] == '\r' && r.buffer[r.parsePosition+1] == '\n' {
		r.parsePosition += 2
		return nil
	}
	return ExpectNewLine
}
func (r *RedisParser) parseBinary() (*Command, error) {
	r.parsePosition++
	numArg, err := r.readNumber()
	if err != nil {
		return nil, err
	}
	var e error
	if e = r.discardNewLine(); e != nil {
		return nil, e
	}
	switch {
	case numArg == -1:
		return nil, r.discardNewLine() // null array
	case numArg < -1:
		return nil, InvalidNumArg
	case numArg > MaxNumArg:
		return nil, InvalidNumArg
	}
	argv := make([][]byte, 0, numArg)
	for i := 0; i < numArg; i++ {
		if e := r.requireNBytes(1); e != nil {
			return nil, e
		}
		if r.buffer[r.parsePosition] != '$' {
			return nil, ExpectTypeChar
		}
		r.parsePosition++
		var plen int
		if plen, e = r.readNumber(); e != nil {
			return nil, e
		}
		if e = r.discardNewLine(); e != nil {
			return nil, e
		}
		switch {
		case plen == -1:
			argv = append(argv, nil) // null bulk
		case plen == 0:
			argv = append(argv, emptyBulk[:]) // empty bulk
		case plen > 0 && plen <= MaxBulkSize:
			if e = r.requireNBytes(plen); e != nil {
				return nil, e
			}
			argv = append(argv, r.buffer[r.parsePosition:(r.parsePosition+plen)])
			r.parsePosition += plen
		default:
			return nil, InvalidBulkSize
		}
		if e = r.discardNewLine(); e != nil {
			return nil, e
		}
	}
	return &Command{argv}, nil
}
func (r *RedisParser) parseTelnet() (*Command, error) {
	nlPos := -1
	for {
		nlPos = bytes.IndexByte(r.buffer, '\n')
		if nlPos == -1 {
			if e := r.readSome(1); e != nil {
				return nil, e
			}
		} else {
			break
		}
		if r.writeIndex > MaxTelnetLine {
			return nil, LineTooLong
		}
	}
	r.reset()
	return &Command{bytes.Split(r.buffer[:nlPos-1], spaceSlice)}, nil
}

func (r *RedisParser) reset() {
	r.writeIndex = 0
	r.parsePosition = 0
}

func (r *RedisParser) ReadCommand() (*Command, error) {
	if err := r.readSome(1); err != nil {
		return nil, err
	}
	var cmd *Command
	var err error
	if r.buffer[r.parsePosition] == '*' {
		cmd, err = r.parseBinary()
	} else {
		cmd, err = r.parseTelnet()
	}
	r.reset()
	return cmd, err
}

var (
	newLine  = []byte{'\r', '\n'}
	nilBulk  = []byte{'$', '-', '1', '\r', '\n'}
	nilArray = []byte{'*', '-', '1', '\r', '\n'}
	okResp   = []byte{'+', 'O', 'K', '\r', '\n'}
)

func intToString(val int64) string {
	return strconv.FormatInt(val, 10)
}
func SendError(w *bufio.Writer, msg string) error {
	resp := "-" + msg + "\r\n"
	_, e := w.Write([]byte(resp))
	if e != nil {
		return e
	}
	return w.Flush()
}
func SendOk(w *bufio.Writer) error {
	_, e := w.Write(okResp)
	if e != nil {
		return e
	}
	return w.Flush()
}

func SendString(w *bufio.Writer, msg string) error {
	resp := "+" + msg + "\r\n"
	_, e := w.Write([]byte(resp))
	if e != nil {
		return e
	}
	return w.Flush()
}

func SendInt(w *bufio.Writer, val int64) error {
	resp := ":" + intToString(val) + "\r\n"
	_, e := w.Write([]byte(resp))
	if e != nil {
		return e
	}
	return w.Flush()
}
func SendBulk(w *bufio.Writer, val []byte) error {
	if e := sendBulk(w, val); e != nil {
		return e
	}
	return w.Flush()
}
func sendBulk(w *bufio.Writer, val []byte) error {
	if val == nil {
		_, e := w.Write(nilBulk)
		if e != nil {
			return e
		}
		return nil
	}
	pre := "$" + intToString(int64(len(val))) + "\r\n"
	_, e := w.Write([]byte(pre))
	if e != nil {
		return e
	}
	_, e = w.Write(val)
	if e != nil {
		return e
	}
	_, e = w.Write(newLine)
	if e != nil {
		return e
	}
	return nil
}
func SendBulks(w *bufio.Writer, vals [][]byte) error {
	if e := sendBulks(w, vals); e != nil {
		return e
	}
	return w.Flush()
}
func sendBulks(w *bufio.Writer, vals [][]byte) error {
	var e error
	if vals == nil {
		_, e = w.Write(nilArray)
		e = w.Flush()
		return e
	}
	pre := "*" + intToString(int64(len(vals))) + "\r\n"
	_, e = w.Write([]byte(pre))
	if e != nil {
		return e
	}
	numArg := len(vals)
	for i := 0; i < numArg; i++ {
		if e = SendBulk(w, vals[i]); e != nil {
			return e
		}
	}
	e = w.Flush()
	return e
}
func SendBulkString(w *bufio.Writer, str string) error {
	return SendBulk(w, []byte(str))
}
func SendBulkStrings(w *bufio.Writer, strs []string) error {
	if strs == nil {
		return SendBulks(w, nil)
	}
	t := make([][]byte, 0, len(strs))
	for i := 0; i < len(strs); i++ {
		t = append(t, []byte(strs[i]))
	}
	return SendBulks(w, t)
}
