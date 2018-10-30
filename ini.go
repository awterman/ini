package test

import (
	"bufio"
	"bytes"
	"strings"
	"unicode"
	"fmt"
	"reflect"
	"strconv"
	"time"
	"errors"
)

const (
	TimeLayout = "2006-01-02T15:04:05"
)

var (
	ErrUnsupportedType = errors.New("unsupported type.")
)

func isValidName(s string) bool {
	if s == "" {
		return false
	}

	for i, c := range s {
		if i == 0 && !unicode.IsLetter(c) {
			return false
		}

		if !unicode.IsLetter(c) && !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

func parse(b []byte) (map[string]map[string]string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(b))

	var (
		conf = make(map[string]map[string]string)
		sec = make(map[string]string)
		secName = ""
	)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		if line[0] == ';' || line[0] == '#' {
			continue
		}

		if line[0] == '[' && line[len(line) - 1] == ']' {
			if secName != "" {
				conf[secName] = sec
				sec = make(map[string]string)
			}

			secName = line[1:len(line) - 1]
			if !isValidName(secName) {
				return nil, fmt.Errorf("invalid section: %s", line)
			}
			continue
		}

		fields := strings.Split(line, "=")
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid item: %s", line)
		}

		key := fields[0]
		val := fields[1]
		if !isValidName(key) {
			return nil, fmt.Errorf("invalid item: %s", line)
		}

		if secName == "" {
			return nil, fmt.Errorf("default section is not supported")
		}

		sec[key] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if secName != "" {
		conf[secName] = sec
	}

	return conf, nil
}

func parseString(s string, v reflect.Value) error {
	var (
		x interface{}
		err error
	)

	pkgPath := v.Type().PkgPath()
	name := v.Type().Name()

	switch pkgPath {
	case "time":
		switch name {
		case "Duration":
			x, err = time.ParseDuration(s)
		case "Time":
			x, err = time.Parse(TimeLayout, s)
		default:
			return ErrUnsupportedType
		}
	case "builtin":
		switch {
		case name == "string":
			x = s
		case name == "bool":
			x, err = strconv.ParseBool(s)
		case strings.HasPrefix(name, "int"):
			bitSize, err := strconv.Atoi(s[len("int"):])
			if err != nil {
				return err
			}
			x, err = strconv.ParseInt(s, 10, bitSize)
		case strings.HasPrefix(name, "uint"):
			bitSize, err := strconv.Atoi(s[len("uint"):])
			if err != nil {
				return err
			}
			x, err = strconv.ParseUint(s, 10, bitSize)
		default:
			return ErrUnsupportedType
		}
	default:
		return ErrUnsupportedType
	}

	if err != nil {
		return err
	}
	v.Set(reflect.ValueOf(x))
	return nil
}

func Unmarshal(b []byte, v interface{}) error {
	conf, err := parse(b)
	if err != nil {
		return err
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("v is not a pointer")
	}

	for i := 0; i < rv.NumField(); i++ {
		secValue := rv.Field(i)
		secName := rv.Type().Field(i).Name

		sec, ok := conf[secName]
		if !ok {
			return fmt.Errorf("section not found: %s", secName)
		}

		for j := 0; j < secValue.NumField(); j++ {
			itemValue := secValue.Field(j)
			key := secValue.Type().Field(j).Name

			val, ok := sec[key]
			if !ok {
				return fmt.Errorf("key not found: [%s]->%s", secName, key)
			}

			err := parseString(val, itemValue)
			if err != nil {
				return fmt.Errorf("can't parse string: %v", err)
			}
		}
	}

	return nil
}
