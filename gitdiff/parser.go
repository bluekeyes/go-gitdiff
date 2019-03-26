// TODO(bkeyes): consider exporting the parser type with configuration
// this would enable OID validation, p-value guessing, and prefix stripping
// by allowing users to set or override defaults

	r *bufio.Reader

	eof    bool
	lineno int64
	lines  [3]string
	for err = p.Next(); err == nil; err = p.Next() {
		line := p.Line(0)
			return nil, p.Errorf(0, "patch fragment without header: %s", line)
		if strings.HasPrefix(line, oldFilePrefix) && strings.HasPrefix(p.Line(1), newFilePrefix) {
			newFileLine := p.Line(1)
			if !isMaybeFragmentHeader(p.Line(2)) {

	if err != nil && err != io.EOF {
		return nil, err
	}
	return file, nil
// Next advances the parser by one line. It returns any error encountered while
// reading the line, including io.EOF when the end of stream is reached.
func (p *parser) Next() error {
	if p.eof {
		p.lines[0] = ""
		return io.EOF
	if p.lineno == 0 {
		// on first call to next, need to shift in all lines
		for i := 0; i < len(p.lines)-1; i++ {
			if err := p.shiftLines(); err != nil && err != io.EOF {
				return err
			}
	err := p.shiftLines()
	if err == io.EOF {
		p.eof = p.lines[1] == ""
	} else if err != nil {
		return err
	p.lineno++
func (p *parser) shiftLines() (err error) {
	for i := 0; i < len(p.lines)-1; i++ {
		p.lines[i] = p.lines[i+1]
	p.lines[len(p.lines)-1], err = p.r.ReadString('\n')
// Line returns a line from the parser without advancing it. A delta of 0
// returns the current line, while higher deltas return read-ahead lines. It
// returns an empty string if the delta is higher than the available lines,
// either because of the buffer size or because the parser reached the end of
// the input. Valid lines always contain at least a newline character.
func (p *parser) Line(delta uint) string {
	return p.lines[delta]
func (p *parser) Errorf(delta int64, msg string, args ...interface{}) error {
	return fmt.Errorf("gitdiff: line %d: %s", p.lineno+delta, fmt.Sprintf(msg, args...))
// TODO(bkeyes): fix duplication with isMaybeFragmentHeader
func parseFragmentHeader(f *Fragment, header string) (err error) {
	const startMark = "@@ "
	const endMark = " @@"
	parts := strings.SplitAfterN(header, endMark, 2)
	if len(parts) < 2 || !strings.HasPrefix(parts[0], startMark) || !strings.HasSuffix(parts[0], endMark) {
		return fmt.Errorf("invalid fragment header")
	header = parts[0][len(startMark) : len(parts[0])-len(endMark)]
	f.Comment = strings.TrimSpace(parts[1])
	ranges := strings.Split(header, " ")
	if len(ranges) != 2 {
		return fmt.Errorf("invalid fragment header")
	if !strings.HasPrefix(ranges[0], "-") || !strings.HasPrefix(ranges[1], "+") {
		return fmt.Errorf("invalid fragment header: bad range marker")
	if f.OldPosition, f.OldLines, err = parseRange(ranges[0][1:]); err != nil {
		return fmt.Errorf("invalid fragment header: %v", err)
	if f.NewPosition, f.NewLines, err = parseRange(ranges[1][1:]); err != nil {
		return fmt.Errorf("invalid fragment header: %v", err)
func parseRange(s string) (start int64, end int64, err error) {
	parts := strings.SplitN(s, ",", 2)
	if start, err = strconv.ParseInt(parts[0], 10, 64); err != nil {
		return 0, 0, fmt.Errorf("bad start of range: %s: %v", parts[0], nerr.Err)
	if len(parts) > 1 {
		if end, err = strconv.ParseInt(parts[1], 10, 64); err != nil {
			nerr := err.(*strconv.NumError)
			return 0, 0, fmt.Errorf("bad end of range: %s: %v", parts[1], nerr.Err)
		end = 1
	return