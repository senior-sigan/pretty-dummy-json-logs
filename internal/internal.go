package internal

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
)

const timeFormat = time.DateTime

var DefaultPalette = Palette{
	KeyColor:              color.New(color.FgGreen),
	ValColor:              color.New(color.FgHiWhite),
	TimeColor:             color.New(color.FgWhite),
	CallerColor:           color.New(color.FgBlue),
	MsgLightBgColor:       color.New(color.FgBlack),
	MsgAbsentLightBgColor: color.New(color.FgHiBlack),
	MsgDarkBgColor:        color.New(color.FgHiWhite),
	MsgAbsentDarkBgColor:  color.New(color.FgWhite),
	DebugLevelColor:       color.New(color.FgMagenta),
	InfoLevelColor:        color.New(color.FgCyan),
	WarnLevelColor:        color.New(color.FgYellow),
	ErrorLevelColor:       color.New(color.FgRed),
	PanicLevelColor:       color.New(color.BgRed),
	FatalLevelColor:       color.New(color.BgHiRed, color.FgHiWhite),
	UnknownLevelColor:     color.New(color.FgMagenta),
}

type Palette struct {
	KeyColor              *color.Color
	ValColor              *color.Color
	TimeColor             *color.Color
	CallerColor           *color.Color
	MsgLightBgColor       *color.Color
	MsgAbsentLightBgColor *color.Color
	MsgDarkBgColor        *color.Color
	MsgAbsentDarkBgColor  *color.Color
	DebugLevelColor       *color.Color
	InfoLevelColor        *color.Color
	WarnLevelColor        *color.Color
	ErrorLevelColor       *color.Color
	PanicLevelColor       *color.Color
	FatalLevelColor       *color.Color
	UnknownLevelColor     *color.Color
}

func Scan(ctx context.Context, src io.Reader) error {
	in := bufio.NewScanner(src)
	in.Buffer(make([]byte, 1024*1024), 1024*1024)
	in.Split(bufio.ScanLines)
	var line uint64

	for in.Scan() {
		line++

		lineData := in.Bytes()
		data := Structured{KVs: make([]KV, 0)}
		ev := Event{Structured: &data, Raw: string(lineData)}

		switch {
		case TryHandleJson(lineData, &data):
		default:
			ev.Structured = nil
		}

		if err := PrettyPrint(ctx, &ev); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}

	return nil
}

func PrettyPrint(ctx context.Context, ev *Event) error {
	if ev.Structured == nil {
		log.Print(ev.Raw)
		return nil
	}

	data := ev.Structured
	lvl := strings.ToUpper(data.Level)[:imin(4, len(data.Level))]
	switch strings.ToLower(data.Level) {
	case "debug":
		lvl = DefaultPalette.DebugLevelColor.Sprint(lvl)
	case "info":
		lvl = DefaultPalette.InfoLevelColor.Sprint(lvl)
	case "warn", "warning":
		lvl = DefaultPalette.WarnLevelColor.Sprint(lvl)
	case "error":
		lvl = DefaultPalette.ErrorLevelColor.Sprint(lvl)
	case "fatal", "panic":
		lvl = DefaultPalette.FatalLevelColor.Sprint(lvl)
	default:
		lvl = DefaultPalette.UnknownLevelColor.Sprint(lvl)
	}

	errorValue := ""
	caller := ""
	kvs := make([]string, 0, len(data.KVs))
	for _, kv := range data.KVs {
		k, v := kv.Key, kv.Value
		if k == "stacktrace" {
			errorValue = kv.Value.(string)
			continue
		}
		if k == "caller" {
			caller = kv.Value.(string)
			continue
		}
		kstr := DefaultPalette.KeyColor.Sprint(k)
		vstr := DefaultPalette.ValColor.Sprint(v)
		kvs = append(kvs, kstr+"="+vstr)
	}
	sort.Strings(kvs)

	log.Printf("%s [%s] %s\t[%s] %s",
		DefaultPalette.TimeColor.Sprint(data.Time.Format(timeFormat)), lvl, data.Msg, DefaultPalette.CallerColor.Sprint(caller), strings.Join(kvs, "\t"))
	if errorValue != "" {
		log.Print(DefaultPalette.ErrorLevelColor.Sprint("╭────────────────Traceback──────────"))
		for _, line := range strings.Split(errorValue, "\n") {
			log.Print(DefaultPalette.ErrorLevelColor.Sprint("│") + line)
		}
		log.Print(DefaultPalette.ErrorLevelColor.Sprint("╰───────────────────────────────────"))
	}

	return nil
}

type Event struct {
	Structured *Structured
	Raw        string
}

type KV struct {
	Key   string
	Value interface{}
}

type Structured struct {
	Time  time.Time
	Msg   string
	Level string
	KVs   []KV
}

func TryHandleJson(d []byte, out *Structured) bool {
	raw := make(map[string]interface{})
	err := json.Unmarshal(d, &raw)
	if err != nil {
		return false
	}

	if time, ok := tryParseTime(raw["ts"]); ok {
		out.Time = time
		delete(raw, "ts")
	}
	if msg, ok := raw["msg"].(string); ok {
		out.Msg = msg
		delete(raw, "msg")
	}
	if level, ok := raw["level"].(string); ok {
		out.Level = level
		delete(raw, "level")
	}

	for k, v := range raw {
		out.KVs = append(out.KVs, KV{Key: k, Value: v})
	}

	return true
}

var formats = []string{
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05-0700",
	time.RFC3339,
	time.RFC3339Nano,
	time.RFC822,
	time.RFC822Z,
	time.RFC850,
	time.RFC1123,
	time.RFC1123Z,
	time.UnixDate,
	time.RubyDate,
	time.ANSIC,
	time.Kitchen,
	time.Stamp,
	time.StampMilli,
	time.StampMicro,
	time.StampNano,
	"2006/01/02 15:04:05",
	"2006/01/02 15:04:05.999999999",
}

func parseTimeFloat64(value float64) time.Time {
	v := int64(value)
	switch {
	case v > 1e18:
	case v > 1e15:
		v *= 1e3
	case v > 1e12:
		v *= 1e6
	default:
		return time.Unix(v, 0)
	}

	return time.Unix(v/1e9, v%1e9)
}

// tries to parse time using a couple of formats before giving up
func tryParseTime(value interface{}) (time.Time, bool) {
	var t time.Time
	var err error
	switch value.(type) {
	case string:
		for _, layout := range formats {
			t, err = time.Parse(layout, value.(string))
			if err == nil {
				return t, true
			}
		}
	case float32:
		return parseTimeFloat64(float64(value.(float32))), true
	case float64:
		return parseTimeFloat64(value.(float64)), true
	case int:
		return parseTimeFloat64(float64(value.(int))), true
	case int32:
		return parseTimeFloat64(float64(value.(int32))), true
	case int64:
		return parseTimeFloat64(float64(value.(int64))), true
	}
	return t, false
}

func imin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
