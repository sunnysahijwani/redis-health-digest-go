package collector

import (
	"strconv"
	"strings"
)

// Info is the parsed result of the Redis INFO command: a flat map of fields
// plus the expanded Keyspace section indexed by DB number.
type Info struct {
	Fields   map[string]string
	Keyspace map[int]map[string]int64
}

// ParseInfo turns the raw multi-line INFO payload into an Info. Section headers
// (`# Server`), blank lines, and malformed lines are ignored. Keyspace lines
// such as `db5:keys=104,expires=0,avg_ttl=0` are expanded into Keyspace[5].
func ParseInfo(raw string) Info {
	info := Info{
		Fields:   make(map[string]string),
		Keyspace: make(map[int]map[string]int64),
	}

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		if db, ok := parseDBKey(key); ok {
			info.Keyspace[db] = parseKeyspaceLine(value)
			continue
		}

		info.Fields[key] = value
	}

	return info
}

// Str returns a field as a string (empty if absent).
func (i Info) Str(key string) string { return i.Fields[key] }

// Int returns a field parsed as int64 (0 if absent or non-numeric).
func (i Info) Int(key string) int64 {
	n, _ := strconv.ParseInt(i.Fields[key], 10, 64)
	return n
}

// Float returns a field parsed as float64; ok is false if absent or unparsable.
func (i Info) Float(key string) (value float64, ok bool) {
	raw, present := i.Fields[key]
	if !present || raw == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// Keys returns the key count for the given DB index (0 if that DB is empty).
func (i Info) Keys(db int) int64 {
	if fields, ok := i.Keyspace[db]; ok {
		return fields["keys"]
	}
	return 0
}

// parseDBKey reports whether key looks like "db<N>" and returns N.
func parseDBKey(key string) (int, bool) {
	if !strings.HasPrefix(key, "db") {
		return 0, false
	}
	n, err := strconv.Atoi(key[2:])
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseKeyspaceLine(value string) map[string]int64 {
	fields := make(map[string]int64)
	for _, pair := range strings.Split(value, ",") {
		name, raw, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			continue
		}
		fields[name] = n
	}
	return fields
}
