package mbt

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

var errXML = errors.New("etree: invalid XML format")
type readSettings struct {
	CharsetReader func(charset string, input io.Reader) (io.Reader, error)
	Permissive bool
}
func newReadSettings() readSettings {
	return readSettings{}
}
type writeSettings struct {
	CanonicalEndTags bool
	CanonicalText bool
	CanonicalAttrVal bool
}
func newWriteSettings() writeSettings {
	return writeSettings{
		CanonicalEndTags: false,
		CanonicalText:    false,
		CanonicalAttrVal: false,
	}
}
type token interface {
	Parent() *element
	dup(parent *element) token
	setParent(parent *element)
	writeTo(w *bufio.Writer, s *writeSettings)
}
type document struct {
	element
	ReadSettings  readSettings
	WriteSettings writeSettings
}
type element struct {
	Space, Tag string
	Attr       []attr
	Child      []token
	parent     *element
}
type attr struct {
	Space, Key string
	Value      string
}
type charData struct {
	Data       string
	parent     *element
	whitespace bool
}
type comment struct {
	Data   string
	parent *element
}
type directive struct {
	Data   string
	parent *element
}
type procInst struct {
	Target string
	Inst   string
	parent *element
}
func newDocument() *document {
	return &document{
		element{Child: make([]token, 0)},
		newReadSettings(),
		newWriteSettings(),
	}
}
func (d *document) Copy() *document {
	return &document{*(d.dup(nil).(*element)), d.ReadSettings, d.WriteSettings}
}
func (d *document) Root() *element {
	for _, t := range d.Child {
		if c, ok := t.(*element); ok {
			return c
		}
	}
	return nil
}
func (d *document) SetRoot(e *element) {
	if e.parent != nil {
		e.parent.RemoveChild(e)
	}
	e.setParent(&d.element)
	for i, t := range d.Child {
		if _, ok := t.(*element); ok {
			t.setParent(nil)
			d.Child[i] = e
			return
		}
	}
	d.Child = append(d.Child, e)
}
func (d *document) ReadFrom(r io.Reader) (n int64, err error) {
	return d.element.readFrom(r, d.ReadSettings)
}
func (d *document) ReadFromFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = d.ReadFrom(f)
	return err
}
func (d *document) ReadFromBytes(b []byte) error {
	_, err := d.ReadFrom(bytes.NewReader(b))
	return err
}
func (d *document) ReadFromString(s string) error {
	_, err := d.ReadFrom(strings.NewReader(s))
	return err
}
func (d *document) WriteTo(w io.Writer) (n int64, err error) {
	cw := newCountWriter(w)
	b := bufio.NewWriter(cw)
	for _, c := range d.Child {
		c.writeTo(b, &d.WriteSettings)
	}
	err, n = b.Flush(), cw.bytes
	return
}
func (d *document) WriteToFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = d.WriteTo(f)
	return err
}
func (d *document) WriteToBytes() (b []byte, err error) {
	var buf bytes.Buffer
	if _, err = d.WriteTo(&buf); err != nil {
		return
	}
	return buf.Bytes(), nil
}
func (d *document) WriteToString() (s string, err error) {
	var b []byte
	if b, err = d.WriteToBytes(); err != nil {
		return
	}
	return string(b), nil
}
type indentFunc func(depth int) string
func (d *document) Indent(spaces int) {
	var indent indentFunc
	switch {
	case spaces < 0:
		indent = func(depth int) string { return "" }
	default:
		indent = func(depth int) string { return crIndent(depth*spaces, crsp) }
	}
	d.element.indent(0, indent)
}
func (d *document) IndentTabs() {
	indent := func(depth int) string { return crIndent(depth, crtab) }
	d.element.indent(0, indent)
}
func newElement(space, tag string, parent *element) *element {
	e := &element{
		Space:  space,
		Tag:    tag,
		Attr:   make([]attr, 0),
		Child:  make([]token, 0),
		parent: parent,
	}
	if parent != nil {
		parent.addChild(e)
	}
	return e
}
func (e *element) Copy() *element {
	var parent *element
	return e.dup(parent).(*element)
}
func (e *element) Text() string {
	if len(e.Child) == 0 {
		return ""
	}
	text := ""
	for _, ch := range e.Child {
		if cd, ok := ch.(*charData); ok {
			if text == "" {
				text = cd.Data
			} else {
				text = text + cd.Data
			}
		} else {
			break
		}
	}
	return text
}
func (e *element) SetText(text string) {
	if len(e.Child) > 0 {
		if cd, ok := e.Child[0].(*charData); ok {
			cd.Data = text
			return
		}
	}
	cd := newCharData(text, false, e)
	copy(e.Child[1:], e.Child[0:])
	e.Child[0] = cd
}
func (e *element) CreateElement(tag string) *element {
	space, stag := spaceDecompose(tag)
	return newElement(space, stag, e)
}
func (e *element) AddChild(t token) {
	if t.Parent() != nil {
		t.Parent().RemoveChild(t)
	}
	t.setParent(e)
	e.addChild(t)
}
func (e *element) InsertChild(ex token, t token) {
	if t.Parent() != nil {
		t.Parent().RemoveChild(t)
	}
	t.setParent(e)
	for i, c := range e.Child {
		if c == ex {
			e.Child = append(e.Child, nil)
			copy(e.Child[i+1:], e.Child[i:])
			e.Child[i] = t
			return
		}
	}
	e.addChild(t)
}
func (e *element) RemoveChild(t token) token {
	for i, c := range e.Child {
		if c == t {
			e.Child = append(e.Child[:i], e.Child[i+1:]...)
			c.setParent(nil)
			return t
		}
	}
	return nil
}
func (e *element) readFrom(ri io.Reader, settings readSettings) (n int64, err error) {
	r := newCountReader(ri)
	dec := xml.NewDecoder(r)
	dec.CharsetReader = settings.CharsetReader
	dec.Strict = !settings.Permissive
	var sta stack
	sta.push(e)
	for {
		t, err := dec.RawToken()
		switch {
		case err == io.EOF:
			return r.bytes, nil
		case err != nil:
			return r.bytes, err
		case sta.empty():
			return r.bytes, errXML
		}
		top := sta.peek().(*element)
		switch t := t.(type) {
		case xml.StartElement:
			e := newElement(t.Name.Space, t.Name.Local, top)
			for _, a := range t.Attr {
				e.createAttr(a.Name.Space, a.Name.Local, a.Value)
			}
			sta.push(e)
		case xml.EndElement:
			sta.pop()
		case xml.CharData:
			data := string(t)
			newCharData(data, isWhitespace(data), top)
		case xml.Comment:
			newComment(string(t), top)
		case xml.Directive:
			newDirective(string(t), top)
		case xml.ProcInst:
			newProcInst(t.Target, string(t.Inst), top)
		}
	}
}
func (e *element) SelectAttr(key string) *attr {
	space, skey := spaceDecompose(key)
	for i, a := range e.Attr {
		if spaceMatch(space, a.Space) && skey == a.Key {
			return &e.Attr[i]
		}
	}
	return nil
}
func (e *element) SelectAttrValue(key, dflt string) string {
	space, skey := spaceDecompose(key)
	for _, a := range e.Attr {
		if spaceMatch(space, a.Space) && skey == a.Key {
			return a.Value
		}
	}
	return dflt
}
func (e *element) ChildElements() []*element {
	var elements []*element
	for _, t := range e.Child {
		if c, ok := t.(*element); ok {
			elements = append(elements, c)
		}
	}
	return elements
}
func (e *element) SelectElement(tag string) *element {
	space, stag := spaceDecompose(tag)
	for _, t := range e.Child {
		if c, ok := t.(*element); ok && spaceMatch(space, c.Space) && stag == c.Tag {
			return c
		}
	}
	return nil
}
func (e *element) SelectElements(tag string) []*element {
	space, stag := spaceDecompose(tag)
	var elements []*element
	for _, t := range e.Child {
		if c, ok := t.(*element); ok && spaceMatch(space, c.Space) && stag == c.Tag {
			elements = append(elements, c)
		}
	}
	return elements
}
func (e *element) FindElement(path string) *element {
	return e.FindElementPath(mustCompilePath(path))
}
func (e *element) FindElementPath(pp path) *element {
	p := newPather()
	elements := p.traverse(e, pp)
	switch {
	case len(elements) > 0:
		return elements[0]
	default:
		return nil
	}
}
func (e *element) FindElements(path string) []*element {
	return e.FindElementsPath(mustCompilePath(path))
}
func (e *element) FindElementsPath(pp path) []*element {
	p := newPather()
	return p.traverse(e, pp)
}
func (e *element) GetPath() string {
	list := make([]string,0)
	for seg := e; seg != nil; seg = seg.Parent() {
		if seg.Tag != "" {
			list = append(list, seg.Tag)
		}
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return "/" + strings.Join(list, "/")
}
func (e *element) GetRelativePath(source *element) string {
	var list []*element
	if source == nil {
		return ""
	}
	var seg *element
	for seg = e; seg != nil && seg != source; seg = seg.Parent() {
		list = append(list, seg)
	}
	if seg == source {
		if len(list) == 0 {
			return "."
		}
		parts := make([]string,0)
		for i := len(list) - 1; i >= 0; i-- {
			parts = append(parts, list[i].Tag)
		}
		return "./" + strings.Join(parts, "/")
	}
	findPathIndex := func(e *element, path []*element) int {
		for i, ee := range path {
			if e == ee {
				return i
			}
		}
		return -1
	}
	climb := 0
	for seg = source; seg != nil; seg = seg.Parent() {
		i := findPathIndex(seg, list)
		if i >= 0 {
			list = list[:i]
			break
		}
		climb++
	}
	if seg == nil {
		return ""
	}
	parts := make([]string,0)
	for i := 0; i < climb; i++ {
		parts = append(parts, "..")
	}
	for i := len(list) - 1; i >= 0; i-- {
		parts = append(parts, list[i].Tag)
	}
	return strings.Join(parts, "/")
}
func (e *element) indent(depth int, indent indentFunc) {
	e.stripIndent()
	n := len(e.Child)
	if n == 0 {
		return
	}
	oldChild := e.Child
	e.Child = make([]token, 0, n*2+1)
	isCharData, firstNonCharData := false, true
	for _, c := range oldChild {
		_, isCharData = c.(*charData)
		if !isCharData {
			if !firstNonCharData || depth > 0 {
				newCharData(indent(depth), true, e)
			}
			firstNonCharData = false
		}
		e.addChild(c)
		if ce, ok := c.(*element); ok {
			ce.indent(depth+1, indent)
		}
	}
	if !isCharData {
		if !firstNonCharData || depth > 0 {
			newCharData(indent(depth-1), true, e)
		}
	}
}
func (e *element) stripIndent() {
	n := len(e.Child)
	for _, c := range e.Child {
		if cd, ok := c.(*charData); ok && cd.whitespace {
			n--
		}
	}
	if n == len(e.Child) {
		return
	}
	newChild := make([]token, n)
	j := 0
	for _, c := range e.Child {
		if cd, ok := c.(*charData); ok && cd.whitespace {
			continue
		}
		newChild[j] = c
		j++
	}
	e.Child = newChild
}
func (e *element) dup(parent *element) token {
	ne := &element{
		Space:  e.Space,
		Tag:    e.Tag,
		Attr:   make([]attr, len(e.Attr)),
		Child:  make([]token, len(e.Child)),
		parent: parent,
	}
	for i, t := range e.Child {
		ne.Child[i] = t.dup(ne)
	}
	for i, a := range e.Attr {
		ne.Attr[i] = a
	}
	return ne
}
func (e *element) Parent() *element {
	return e.parent
}
func (e *element) setParent(parent *element) {
	e.parent = parent
}
func (e *element) writeTo(w *bufio.Writer, s *writeSettings) {
	w.WriteByte('<')
	if e.Space != "" {
		w.WriteString(e.Space)
		w.WriteByte(':')
	}
	w.WriteString(e.Tag)
	for _, a := range e.Attr {
		w.WriteByte(' ')
		a.writeTo(w, s)
	}
	if len(e.Child) > 0 {
		w.WriteString(">")
		for _, c := range e.Child {
			c.writeTo(w, s)
		}
		w.Write([]byte{'<', '/'})
		if e.Space != "" {
			w.WriteString(e.Space)
			w.WriteByte(':')
		}
		w.WriteString(e.Tag)
		w.WriteByte('>')
	} else {
		if s.CanonicalEndTags {
			w.Write([]byte{'>', '<', '/'})
			if e.Space != "" {
				w.WriteString(e.Space)
				w.WriteByte(':')
			}
			w.WriteString(e.Tag)
			w.WriteByte('>')
		} else {
			w.Write([]byte{'/', '>'})
		}
	}
}
func (e *element) addChild(t token) {
	e.Child = append(e.Child, t)
}
func (e *element) CreateAttr(key, value string) *attr {
	space, skey := spaceDecompose(key)
	return e.createAttr(space, skey, value)
}
func (e *element) createAttr(space, key, value string) *attr {
	for i, a := range e.Attr {
		if space == a.Space && key == a.Key {
			e.Attr[i].Value = value
			return &e.Attr[i]
		}
	}
	a := attr{space, key, value}
	e.Attr = append(e.Attr, a)
	return &e.Attr[len(e.Attr)-1]
}
func (e *element) RemoveAttr(key string) *attr {
	space, skey := spaceDecompose(key)
	for i, a := range e.Attr {
		if space == a.Space && skey == a.Key {
			e.Attr = append(e.Attr[0:i], e.Attr[i+1:]...)
			return &a
		}
	}
	return nil
}
func (e *element) SortAttrs() {
	sort.Sort(byAttr(e.Attr))
}
type byAttr []attr
func (a byAttr) Len() int {
	return len(a)
}
func (a byAttr) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a byAttr) Less(i, j int) bool {
	sp := strings.Compare(a[i].Space, a[j].Space)
	if sp == 0 {
		return strings.Compare(a[i].Key, a[j].Key) < 0
	}
	return sp < 0
}
var xmlReplacerNormal = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"'", "&apos;",
	`"`, "&quot;",
)
var xmlReplacerCanonicalText = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"\r", "&#xD;",
)
var xmlReplacerCanonicalAttrVal = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	`"`, "&quot;",
	"\t", "&#x9;",
	"\n", "&#xA;",
	"\r", "&#xD;",
)
func (a *attr) writeTo(w *bufio.Writer, s *writeSettings) {
	if a.Space != "" {
		w.WriteString(a.Space)
		w.WriteByte(':')
	}
	w.WriteString(a.Key)
	w.WriteString(`="`)
	var r *strings.Replacer
	if s.CanonicalAttrVal {
		r = xmlReplacerCanonicalAttrVal
	} else {
		r = xmlReplacerNormal
	}
	w.WriteString(r.Replace(a.Value))
	w.WriteByte('"')
}
func newCharData(data string, whitespace bool, parent *element) *charData {
	c := &charData{
		Data:       data,
		whitespace: whitespace,
		parent:     parent,
	}
	if parent != nil {
		parent.addChild(c)
	}
	return c
}
func (e *element) CreateCharData(data string) *charData {
	return newCharData(data, false, e)
}
func (c *charData) dup(parent *element) token {
	return &charData{
		Data:       c.Data,
		whitespace: c.whitespace,
		parent:     parent,
	}
}
func (c *charData) Parent() *element {
	return c.parent
}
func (c *charData) setParent(parent *element) {
	c.parent = parent
}
func (c *charData) writeTo(w *bufio.Writer, s *writeSettings) {
	var r *strings.Replacer
	if s.CanonicalText {
		r = xmlReplacerCanonicalText
	} else {
		r = xmlReplacerNormal
	}
	w.WriteString(r.Replace(c.Data))
}
func newComment(cc string, parent *element) *comment {
	c := &comment{
		Data:   cc,
		parent: parent,
	}
	if parent != nil {
		parent.addChild(c)
	}
	return c
}
func (e *element) CreateComment(comment string) *comment {
	return newComment(comment, e)
}
func (c *comment) dup(parent *element) token {
	return &comment{
		Data:   c.Data,
		parent: parent,
	}
}
func (c *comment) Parent() *element {
	return c.parent
}
func (c *comment) setParent(parent *element) {
	c.parent = parent
}
func (c *comment) writeTo(w *bufio.Writer, s *writeSettings) {
	w.WriteString("<!--")
	w.WriteString(c.Data)
	w.WriteString("-->")
}

func newDirective(data string, parent *element) *directive {
	d := &directive{
		Data:   data,
		parent: parent,
	}
	if parent != nil {
		parent.addChild(d)
	}
	return d
}
func (e *element) CreateDirective(data string) *directive {
	return newDirective(data, e)
}
func (d *directive) dup(parent *element) token {
	return &directive{
		Data:   d.Data,
		parent: parent,
	}
}
func (d *directive) Parent() *element {
	return d.parent
}
func (d *directive) setParent(parent *element) {
	d.parent = parent
}
func (d *directive) writeTo(w *bufio.Writer, s *writeSettings) {
	w.WriteString("<!")
	w.WriteString(d.Data)
	w.WriteString(">")
}
func newProcInst(target, inst string, parent *element) *procInst {
	p := &procInst{
		Target: target,
		Inst:   inst,
		parent: parent,
	}
	if parent != nil {
		parent.addChild(p)
	}
	return p
}
func (e *element) CreateProcInst(target, inst string) *procInst {
	return newProcInst(target, inst, e)
}
func (p *procInst) dup(parent *element) token {
	return &procInst{
		Target: p.Target,
		Inst:   p.Inst,
		parent: parent,
	}
}
func (p *procInst) Parent() *element {
	return p.parent
}
func (p *procInst) setParent(parent *element) {
	p.parent = parent
}
func (p *procInst) writeTo(w *bufio.Writer, s *writeSettings) {
	w.WriteString("<?")
	w.WriteString(p.Target)
	if p.Inst != "" {
		w.WriteByte(' ')
		w.WriteString(p.Inst)
	}
	w.WriteString("?>")
}
type path struct {
	segments []segment
}
type errPath string
func (err errPath) Error() string {
	return "etree: " + string(err)
}
func compilePath(str string) (path, error) {
	var comp compiler
	segments := comp.parsePath(str)
	if comp.err != errPath("") {
		return path{nil}, comp.err
	}
	return path{segments}, nil
}
func mustCompilePath(path string) path {
	p, err := compilePath(path)
	if err != nil {
		panic(err)
	}
	return p
}
type segment struct {
	sel     selector
	filters []filter
}
func (seg *segment) apply(e *element, p *pather) {
	seg.sel.apply(e, p)
	for _, f := range seg.filters {
		f.apply(p)
	}
}
type selector interface {
	apply(e *element, p *pather)
}
type filter interface {
	apply(p *pather)
}
type pather struct {
	queue      fifo
	results    []*element
	inResults  map[*element]bool
	candidates []*element
	scratch    []*element // used by filters
}
type nodes struct {
	e        *element
	segments []segment
}
func newPather() *pather {
	return &pather{
		results:    make([]*element, 0),
		inResults:  make(map[*element]bool),
		candidates: make([]*element, 0),
		scratch:    make([]*element, 0),
	}
}
func (p *pather) traverse(e *element, path path) []*element {
	for p.queue.add(nodes{e, path.segments}); p.queue.len() > 0; {
		p.eval(p.queue.remove().(nodes))
	}
	return p.results
}
func (p *pather) eval(n nodes) {
	p.candidates = p.candidates[0:0]
	seg, remain := n.segments[0], n.segments[1:]
	seg.apply(n.e, p)

	if len(remain) == 0 {
		for _, c := range p.candidates {
			if in := p.inResults[c]; !in {
				p.inResults[c] = true
				p.results = append(p.results, c)
			}
		}
	} else {
		for _, c := range p.candidates {
			p.queue.add(nodes{c, remain})
		}
	}
}
type compiler struct {
	err errPath
}
func (c *compiler) parsePath(path string) []segment {
	if strings.HasSuffix(path, "//") {
		path = path + "*"
	}
	var segments []segment
	if strings.HasPrefix(path, "/") {
		segments = append(segments, segment{new(selectRoot), []filter{}})
		path = path[1:]
	}
	for _, s := range splitPath(path) {
		segments = append(segments, c.parseSegment(s))
		if c.err != errPath("") {
			break
		}
	}
	return segments
}
func splitPath(path string) []string {
	pieces := make([]string, 0)
	start := 0
	inquote := false
	for i := 0; i+1 <= len(path); i++ {
		if path[i] == '\'' {
			inquote = !inquote
		} else if path[i] == '/' && !inquote {
			pieces = append(pieces, path[start:i])
			start = i + 1
		}
	}
	return append(pieces, path[start:])
}
func (c *compiler) parseSegment(path string) segment {
	pieces := strings.Split(path, "[")
	seg := segment{
		sel:     c.parseSelector(pieces[0]),
		filters: []filter{},
	}
	for i := 1; i < len(pieces); i++ {
		fpath := pieces[i]
		if fpath[len(fpath)-1] != ']' {
			c.err = errPath("path has invalid filter [brackets].")
			break
		}
		seg.filters = append(seg.filters, c.parseFilter(fpath[:len(fpath)-1]))
	}
	return seg
}
func (c *compiler) parseSelector(path string) selector {
	switch path {
	case ".":
		return new(selectSelf)
	case "..":
		return new(selectParent)
	case "*":
		return new(selectChildren)
	case "":
		return new(selectDescendants)
	default:
		return newSelectChildrenByTag(path)
	}
}
func (c *compiler) parseFilter(path string) filter {
	if len(path) == 0 {
		c.err = errPath("path contains an empty filter expression.")
		return nil
	}
	eqindex := strings.Index(path, "='")
	if eqindex >= 0 {
		rindex := nextIndex(path, "'", eqindex+2)
		if rindex != len(path)-1 {
			c.err = errPath("path has mismatched filter quotes.")
			return nil
		}
		switch {
		case path[0] == '@':
			return newFilterAttrVal(path[1:eqindex], path[eqindex+2:rindex])
		case strings.HasPrefix(path, "text()"):
			return newFilterTextVal(path[eqindex+2 : rindex])
		default:
			return newFilterChildText(path[:eqindex], path[eqindex+2:rindex])
		}
	}
	switch {
	case path[0] == '@':
		return newFilterAttr(path[1:])
	case path == "text()":
		return newFilterText()
	case isInteger(path):
		pos, _ := strconv.Atoi(path)
		switch {
		case pos > 0:
			return newFilterPos(pos - 1)
		default:
			return newFilterPos(pos)
		}
	default:
		return newFilterChild(path)
	}
}
type selectSelf struct{}
func (s *selectSelf) apply(e *element, p *pather) {
	p.candidates = append(p.candidates, e)
}
type selectRoot struct{}
func (s *selectRoot) apply(e *element, p *pather) {
	root := e
	for root.parent != nil {
		root = root.parent
	}
	p.candidates = append(p.candidates, root)
}
type selectParent struct{}
func (s *selectParent) apply(e *element, p *pather) {
	if e.parent != nil {
		p.candidates = append(p.candidates, e.parent)
	}
}
type selectChildren struct{}
func (s *selectChildren) apply(e *element, p *pather) {
	for _, c := range e.Child {
		if c, ok := c.(*element); ok {
			p.candidates = append(p.candidates, c)
		}
	}
}
type selectDescendants struct{}
func (s *selectDescendants) apply(e *element, p *pather) {
	var queue fifo
	for queue.add(e); queue.len() > 0; {
		e := queue.remove().(*element)
		p.candidates = append(p.candidates, e)
		for _, c := range e.Child {
			if c, ok := c.(*element); ok {
				queue.add(c)
			}
		}
	}
}
type selectChildrenByTag struct {
	space, tag string
}
func newSelectChildrenByTag(path string) *selectChildrenByTag {
	s, l := spaceDecompose(path)
	return &selectChildrenByTag{s, l}
}
func (s *selectChildrenByTag) apply(e *element, p *pather) {
	for _, c := range e.Child {
		if c, ok := c.(*element); ok && spaceMatch(s.space, c.Space) && s.tag == c.Tag {
			p.candidates = append(p.candidates, c)
		}
	}
}
type filterPos struct {
	index int
}
func newFilterPos(pos int) *filterPos {
	return &filterPos{pos}
}
func (f *filterPos) apply(p *pather) {
	if f.index >= 0 {
		if f.index < len(p.candidates) {
			p.scratch = append(p.scratch, p.candidates[f.index])
		}
	} else {
		if -f.index <= len(p.candidates) {
			p.scratch = append(p.scratch, p.candidates[len(p.candidates)+f.index])
		}
	}
	p.candidates, p.scratch = p.scratch, p.candidates[0:0]
}
type filterAttr struct {
	space, key string
}
func newFilterAttr(str string) *filterAttr {
	s, l := spaceDecompose(str)
	return &filterAttr{s, l}
}
func (f *filterAttr) apply(p *pather) {
	for _, c := range p.candidates {
		for _, a := range c.Attr {
			if spaceMatch(f.space, a.Space) && f.key == a.Key {
				p.scratch = append(p.scratch, c)
				break
			}
		}
	}
	p.candidates, p.scratch = p.scratch, p.candidates[0:0]
}
type filterAttrVal struct {
	space, key, val string
}
func newFilterAttrVal(str, value string) *filterAttrVal {
	s, l := spaceDecompose(str)
	return &filterAttrVal{s, l, value}
}
func (f *filterAttrVal) apply(p *pather) {
	for _, c := range p.candidates {
		for _, a := range c.Attr {
			if spaceMatch(f.space, a.Space) && f.key == a.Key && f.val == a.Value {
				p.scratch = append(p.scratch, c)
				break
			}
		}
	}
	p.candidates, p.scratch = p.scratch, p.candidates[0:0]
}
type filterText struct{}
func newFilterText() *filterText {
	return &filterText{}
}
func (f *filterText) apply(p *pather) {
	for _, c := range p.candidates {
		if c.Text() != "" {
			p.scratch = append(p.scratch, c)
		}
	}
	p.candidates, p.scratch = p.scratch, p.candidates[0:0]
}
type filterTextVal struct {
	val string
}

func newFilterTextVal(value string) *filterTextVal {
	return &filterTextVal{value}
}
func (f *filterTextVal) apply(p *pather) {
	for _, c := range p.candidates {
		if c.Text() == f.val {
			p.scratch = append(p.scratch, c)
		}
	}
	p.candidates, p.scratch = p.scratch, p.candidates[0:0]
}
type filterChild struct {
	space, tag string
}
func newFilterChild(str string) *filterChild {
	s, l := spaceDecompose(str)
	return &filterChild{s, l}
}
func (f *filterChild) apply(p *pather) {
	for _, c := range p.candidates {
		for _, cc := range c.Child {
			if cc, ok := cc.(*element); ok &&
				spaceMatch(f.space, cc.Space) &&
				f.tag == cc.Tag {
				p.scratch = append(p.scratch, c)
			}
		}
	}
	p.candidates, p.scratch = p.scratch, p.candidates[0:0]
}
type filterChildText struct {
	space, tag, text string
}
func newFilterChildText(str, text string) *filterChildText {
	s, l := spaceDecompose(str)
	return &filterChildText{s, l, text}
}
func (f *filterChildText) apply(p *pather) {
	for _, c := range p.candidates {
		for _, cc := range c.Child {
			if cc, ok := cc.(*element); ok &&
				spaceMatch(f.space, cc.Space) &&
				f.tag == cc.Tag &&
				f.text == cc.Text() {
				p.scratch = append(p.scratch, c)
			}
		}
	}
	p.candidates, p.scratch = p.scratch, p.candidates[0:0]
}
type stack struct {
	data []interface{}
}
func (s *stack) empty() bool {
	return len(s.data) == 0
}
func (s *stack) push(value interface{}) {
	s.data = append(s.data, value)
}
func (s *stack) pop() interface{} {
	value := s.data[len(s.data)-1]
	s.data[len(s.data)-1] = nil
	s.data = s.data[:len(s.data)-1]
	return value
}
func (s *stack) peek() interface{} {
	return s.data[len(s.data)-1]
}
type fifo struct {
	data       []interface{}
	head, tail int
}
func (f *fifo) add(value interface{}) {
	if f.len()+1 >= len(f.data) {
		f.grow()
	}
	f.data[f.tail] = value
	if f.tail++; f.tail == len(f.data) {
		f.tail = 0
	}
}
func (f *fifo) remove() interface{} {
	value := f.data[f.head]
	f.data[f.head] = nil
	if f.head++; f.head == len(f.data) {
		f.head = 0
	}
	return value
}
func (f *fifo) len() int {
	if f.tail >= f.head {
		return f.tail - f.head
	}
	return len(f.data) - f.head + f.tail
}
func (f *fifo) grow() {
	c := len(f.data) * 2
	if c == 0 {
		c = 4
	}
	buf, count := make([]interface{}, c), f.len()
	if f.tail >= f.head {
		copy(buf[0:count], f.data[f.head:f.tail])
	} else {
		hindex := len(f.data) - f.head
		copy(buf[0:hindex], f.data[f.head:])
		copy(buf[hindex:count], f.data[:f.tail])
	}
	f.data, f.head, f.tail = buf, 0, count
}
type countReader struct {
	r     io.Reader
	bytes int64
}
func newCountReader(r io.Reader) *countReader {
	return &countReader{r: r}
}
func (cr *countReader) Read(p []byte) (n int, err error) {
	b, err := cr.r.Read(p)
	cr.bytes += int64(b)
	return b, err
}
type countWriter struct {
	w     io.Writer
	bytes int64
}
func newCountWriter(w io.Writer) *countWriter {
	return &countWriter{w: w}
}
func (cw *countWriter) Write(p []byte) (n int, err error) {
	b, err := cw.w.Write(p)
	cw.bytes += int64(b)
	return b, err
}
func isWhitespace(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}
	return true
}
func spaceMatch(a, b string) bool {
	switch {
	case a == "":
		return true
	default:
		return a == b
	}
}
func spaceDecompose(str string) (space, key string) {
	colon := strings.IndexByte(str, ':')
	if colon == -1 {
		return "", str
	}
	return str[:colon], str[colon+1:]
}
const (
	crsp  = "\n                                                                "
	crtab = "\n\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t"
)
func crIndent(n int, source string) string {
	switch {
	case n < 0:
		return source[:1]
	case n < len(source):
		return source[:n+1]
	default:
		return source + strings.Repeat(source[1:2], n-len(source)+1)
	}
}
func nextIndex(s, sep string, offset int) int {
	switch i := strings.Index(s[offset:], sep); i {
	case -1:
		return -1
	default:
		return offset + i
	}
}
func isInteger(s string) bool {
	for i := 0; i < len(s); i++ {
		if (s[i] < '0' || s[i] > '9') && !(i == 0 && s[i] == '-') {
			return false
		}
	}
	return true
}

