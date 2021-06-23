package example

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)
var (
	patternList = []*regexp.Regexp{
		regexp.MustCompile(`%[%+A-Za-z]`),
		regexp.MustCompile(`\*+`),
	}
	directives = map[byte]appender{
		'A': timefmt("Monday"),
		'a': timefmt("Mon"),
		'B': timefmt("January"),
		'b': timefmt("Jan"),
		'C': &century{},
		'c': timefmt("Mon Jan _2 15:04:05 2006"),
		'D': timefmt("01/02/06"),
		'd': timefmt("02"),
		'e': timefmt("_2"),
		'F': timefmt("2006-01-02"),
		'H': timefmt("15"),
		'h': timefmt("Jan"),
		'I': timefmt("3"),
		'j': &dayofyear{},
		'k': hourwblank(false),
		'l': hourwblank(true),
		'M': timefmt("04"),
		'm': timefmt("01"),
		'n': verbatim("\n"),
		'p': timefmt("PM"),
		'R': timefmt("15:04"),
		'r': timefmt("3:04:05 PM"),
		'S': timefmt("05"),
		'T': timefmt("15:04:05"),
		't': verbatim("\t"),
		'U': weeknumberOffset(0),
		'u': weekday(1),
		'V': &weeknumber{},
		'v': timefmt("_2-Jan-2006"),
		'W': weeknumberOffset(1),
		'w': weekday(0),
		'X': timefmt("15:04:05"),
		'x': timefmt("01/02/06"),
		'Y': timefmt("2006"),
		'y': timefmt("06"),
		'Z': timefmt("MST"),
		'z': timefmt("-0700"),
		'%': verbatim("%"),
	}
	combineExclusion = []string{
		"Mon",
		"Monday",
		"Jan",
		"January",
		"MST",
		"PM",
	}
	local = clockFn(time.Now)
	utc = clockFn(func() time.Time { return time.Now().UTC() })
)
type (
	rotateLog struct {
		clock       clock
		curFn       string
		globPattern string
		linkName    string
		maxAge      time.Duration
		interval    time.Duration
		mu          sync.RWMutex
		pattern     *strTime
		outFh       *os.File
		count       int
	}
	clock interface {
		Now() time.Time
	}
	clockFn func() time.Time
	option interface {
		Configure(*rotateLog)error
	}
	optionFn func(*rotateLog) error
	cleanupGuard struct {
		enable bool
		fn     func()
		mutex  sync.Mutex
	}
	strTime struct {
		pattern  string
		complied appendList
	}
	combineAppend struct {
		list  appendList
		prev  appender
		isCan bool
	}
	combiner interface {
		canCombine() bool
		combine(combiner) appender
		str() string
	}
	appender interface {
		Append([]byte,time.Time)[]byte
	}
	appendList []appender
	century struct{}
	weekday int
	weeknumberOffset int
	weeknumber struct{}
	hourwblank bool
	dayofyear struct{}
	verbatimw struct {
		s string
	}
	timefmtw struct {
		s string
	}

)

func newStrTime(str string)(*strTime,error){
	var wl appendList
	if err := compile(&wl, str); err != nil {
		return nil, errors.New( `failed to compile format`+err.Error())
	}
	return &strTime{
		pattern:  str,
		complied: wl,
	}, nil
}
//func initLog(pattern string,options ...option)(*rotateLog,error){
//	str := pattern
//	for _,v := range patternList {
//		str = v.ReplaceAllString(str,"*")
//	}
//	obj,err := newStrTime(pattern)
//	if err != nil {
//		return nil, errors.New(`invalid strTime pattern`+err.Error())
//	}
//	var rl rotateLog
//	rl.clock = local
//	rl.globPattern = str
//	rl.pattern = obj
//	rl.interval = 24 * time.Hour
//	rl.maxAge = 7 * 24 * time.Hour
//	rl.count = -1
//	for _, opt := range options {
//		opt.Configure(&rl)
//	}
//	return &rl, nil
//}
func compile(wl *appendList, p string) error {
	var ca combineAppend
	for l := len(p); l > 0; l = len(p) {
		i := strings.IndexByte(p, '%')
		if i < 0 {
			ca.Append(verbatim(p))
			p = p[l:]
			continue
		}
		if i == l-1 {
			return errors.New(`stray % at the end of pattern`)
		}
		if i > 0 {
			ca.Append(verbatim(p[:i]))
			p = p[i:]
		}
		direc, ok := directives[p[1]]
		if !ok {
			return errors.New(fmt.Sprintf(`unknown time format specification '%c'`,p[1]))
		}
		ca.Append(direc)
		p = p[2:]
	}
	*wl = ca.list
	return nil
}
func (ca *combineAppend) Append(w appender) {
	if ca.isCan {
		if wc, ok := w.(combiner); ok && wc.canCombine() {
			ca.prev = ca.prev.(combiner).combine(wc)
			ca.list[len(ca.list)-1] = ca.prev
			return
		}
	}
	ca.list = append(ca.list, w)
	ca.prev = w
	ca.isCan = false
	if comb, ok := w.(combiner); ok {
		if comb.canCombine() {
			ca.isCan = true
		}
	}
}
func (v century) Append(b []byte, t time.Time) []byte {
	n := t.Year() / 100
	if n < 10 {
		b = append(b, '0')
	}
	return append(b, strconv.Itoa(n)...)
}
func (v weekday) Append(b []byte, t time.Time) []byte {
	n := int(t.Weekday())
	if n < int(v) {
		n += 7
	}
	return append(b, byte(n+48))
}
func (v weeknumberOffset) Append(b []byte, t time.Time) []byte {
	yd := t.YearDay()
	offset := int(t.Weekday()) - int(v)
	if offset < 0 {
		offset += 7
	}

	if yd < offset {
		return append(b, '0', '0')
	}

	n := ((yd - offset) / 7) + 1
	if n < 10 {
		b = append(b, '0')
	}
	return append(b, strconv.Itoa(n)...)
}
func (v weeknumber) Append(b []byte, t time.Time) []byte {
	_, n := t.ISOWeek()
	if n < 10 {
		b = append(b, '0')
	}
	return append(b, strconv.Itoa(n)...)
}
func (v dayofyear) Append(b []byte, t time.Time) []byte {
	n := t.YearDay()
	if n < 10 {
		b = append(b, '0', '0')
	} else if n < 100 {
		b = append(b, '0')
	}
	return append(b, strconv.Itoa(n)...)
}
func (v hourwblank) Append(b []byte, t time.Time) []byte {
	h := t.Hour()
	if bool(v) && h > 12 {
		h = h - 12
	}
	if h < 10 {
		b = append(b, ' ')
	}
	return append(b, strconv.Itoa(h)...)
}
func timefmt(s string) *timefmtw {
	return &timefmtw{s: s}
}
func (v timefmtw) Append(b []byte, t time.Time) []byte {
	return t.AppendFormat(b, v.s)
}
func (v timefmtw) str() string {
	return v.s
}
func (v timefmtw) canCombine() bool {
	return true
}
func (v timefmtw) combine(w combiner) appender {
	return timefmt(v.s + w.str())
}
func verbatim(s string) *verbatimw {
	return &verbatimw{s: s}
}
func (v verbatimw) Append(b []byte, _ time.Time) []byte {
	return append(b, v.s...)
}
func (v verbatimw) canCombine() bool {
	return canCombine(v.s)
}
func (v verbatimw) combine(w combiner) appender {
	if _, ok := w.(*timefmtw); ok {
		return timefmt(v.s + w.str())
	}
	return verbatim(v.s + w.str())
}
func (v verbatimw) str() string {
	return v.s
}
func canCombine(s string) bool {
	if strings.ContainsAny(s, "0123456789") {
		return false
	}
	for _, word := range combineExclusion {
		if strings.Contains(s, word) {
			return false
		}
	}
	return true
}
func (f *strTime) format(b []byte, t time.Time) []byte {
	for _, w := range f.complied {
		b = w.Append(b, t)
	}
	return b
}
func (f *strTime) formatStr(t time.Time) string {
	const bufSize = 64
	var b []byte
	max := len(f.pattern) + 10
	if max < bufSize {
		var buf [bufSize]byte
		b = buf[:0]
	} else {
		b = make([]byte, 0, max)
	}
	return string(f.format(b, t))
}
func (c clockFn) Now() time.Time {
	return c()
}

func (o optionFn) Configure(rl *rotateLog) error {
	return o(rl)
}
func withClock(c clock) option {
	return optionFn(func(rl *rotateLog) error {
		rl.clock = c
		return nil
	})
}
// 参数 maxAge,count只能二选其一
func initLog(logPath string,linkName string,interval,maxAge,count int)(*rotateLog,error){
	str := logPath + ".%Y%m%d%H%M"
	for _,v := range patternList {
		str = v.ReplaceAllString(str,"*")
	}
	obj,err := newStrTime(logPath)
	if err != nil {
		return nil, errors.New(`invalid strTime pattern`+err.Error())
	}
	rl := &rotateLog{
		clock:       local,
		linkName:    linkName,
		globPattern: str,
		pattern:     obj,
		interval:    time.Duration(interval) * time.Second,
		maxAge:      time.Duration(maxAge) * time.Second,
		count:       count,
	}
	return rl, nil
}

//func WithLocation(loc *time.Location) option {
//	return withClock(clockFn(func() time.Time {
//		return time.Now().In(loc)
//	}))
//}
////为最新的日志建立软连接
//func WithLinkName(s string) option {
//	return optionFn(func(rl *rotateLog) error {
//		rl.linkName = s
//		return nil
//	})
//}
////设置文件清理前的最长保存时间
//func WithMaxAge(d time.Duration) option {
//	return optionFn(func(rl *rotateLog) error {
//		if rl.count > 0 && d > 0 {
//			return errors.New("attempt to set MaxAge when RotationCount is also given")
//		}
//		rl.maxAge = d
//		return nil
//	})
//}
////设置日志分割的时间，隔多久分割一次
//func WithRotationTime(d time.Duration) option {
//	return optionFn(func(rl *rotateLog) error {
//		rl.interval = d
//		return nil
//	})
//}
// 设置文件清理前最多保存的个数
//func WithRotationCount(n int) option {
//	return optionFn(func(rl *rotateLog) error {
//		if rl.maxAge > 0 && n > 0 {
//			return errors.New("attempt to set RotationCount when MaxAge is also given")
//		}
//		rl.count = n
//		return nil
//	})
//}
func (rl *rotateLog) Write(p []byte) (n int, err error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	out, err := rl.targetWriter()
	if err != nil {
		return 0, errors.New(`failed to acquite target io.Writer`+err.Error())
	}
	return out.Write(p)
}
func (rl *rotateLog) targetWriter() (io.Writer, error) {
	filename := rl.genFilename()
	if rl.curFn == filename {
		return rl.outFh, nil
	}
	fh, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to open file %s: %s", rl.pattern, err))
	}
	if err = rl.rotate(filename); err != nil {
		fmt.Fprintf(os.Stderr, "failed to rotate: %s\n", err)
	}
	rl.outFh.Close()
	rl.outFh = fh
	rl.curFn = filename
	return fh, nil
}
func (g *cleanupGuard) Enable() {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.enable = true
}
func (g *cleanupGuard) Run() {
	g.fn()
}
func (rl *rotateLog) genFilename() string {
	now := rl.clock.Now()
	diff := time.Duration(now.UnixNano()) % rl.interval
	t := now.Add(-1 * diff)
	return rl.pattern.formatStr(t)
}
func (rl *rotateLog) rotate(filename string) error {
	lockfn := filename + `_lock`
	fh, err := os.OpenFile(lockfn, os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		// Can't lock, just return
		return err
	}

	var guard cleanupGuard
	guard.fn = func() {
		fh.Close()
		os.Remove(lockfn)
	}
	defer guard.Run()

	if rl.linkName != "" {
		tmpLinkName := filename + `_symlink`
		if err = os.Symlink(filename, tmpLinkName); err != nil {
			return errors.New(`failed to create new symlink` + err.Error())
		}

		if err = os.Rename(tmpLinkName, rl.linkName); err != nil {
			return errors.New(`failed to rename new symlink`+err.Error())
		}
	}

	if rl.maxAge <= 0 && rl.count <= 0 {
		return errors.New("panic: maxAge and rotationCount are both set")
	}

	matches, err := filepath.Glob(rl.globPattern)
	if err != nil {
		return err
	}
	cutoff := rl.clock.Now().Add(-1 * rl.maxAge)
	var toUnlink []string
	for _, v := range matches {
		if strings.HasSuffix(v, "_lock") || strings.HasSuffix(v, "_symlink") {
			continue
		}
		fi, err := os.Stat(v)
		if err != nil {
			continue
		}
		fl, err := os.Lstat(v)
		if err != nil {
			continue
		}
		if rl.maxAge > 0 && fi.ModTime().After(cutoff) {
			continue
		}
		if rl.count > 0 && fl.Mode()&os.ModeSymlink == os.ModeSymlink {
			continue
		}
		toUnlink = append(toUnlink, v)
	}
	if rl.count > 0 {
		if rl.count >= len(toUnlink) {
			return nil
		}
		toUnlink = toUnlink[:len(toUnlink)-rl.count]
	}
	if len(toUnlink) <= 0 {
		return nil
	}
	guard.Enable()
	go func() {
		for _, v := range toUnlink {
			os.Remove(v)
		}
	}()
	return nil
}