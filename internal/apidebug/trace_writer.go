package apidebug

import "io"

type TraceWriter struct {
	w          io.Writer
	collector  *Collector
	totalBytes int64
	capture    []byte
}

func NewTraceWriter(w io.Writer, c *Collector) *TraceWriter {
	return &TraceWriter{w: w, collector: c}
}

func (tw *TraceWriter) Write(p []byte) (int, error) {
	n, err := tw.w.Write(p)
	if n > 0 {
		tw.totalBytes += int64(n)
		if tw.collector != nil && len(tw.capture) < MaxBodyBytes {
			remain := MaxBodyBytes - len(tw.capture)
			if n <= remain {
				tw.capture = append(tw.capture, p[:n]...)
			} else {
				tw.capture = append(tw.capture, p[:remain]...)
			}
		}
	}
	return n, err
}

func (tw *TraceWriter) RecordDownstreamResponse() {
	if tw == nil || tw.collector == nil {
		return
	}
	body, truncated := TruncateBody(tw.capture)
	note := ""
	if tw.totalBytes > int64(len(tw.capture)) {
		truncated = true
		note = formatByteNote(tw.totalBytes)
	} else if tw.totalBytes > 0 && int64(len(body)) < tw.totalBytes {
		truncated = true
		note = formatByteNote(tw.totalBytes)
	}
	if truncated && note == "" {
		note = "body truncated"
	}
	tw.collector.AddStep(StepInput{
		Name:     "downstream_response",
		Phase:    PhaseResponse,
		BodyText: body,
		Note:     note,
	})
}

func formatByteNote(total int64) string {
	if total <= 0 {
		return ""
	}
	return "total_bytes=" + itoa(total)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
