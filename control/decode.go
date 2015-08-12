/* {{{ Copyright (c) Paul R. Tagliamonte <paultag@debian.org>, 2015
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE. }}} */

package control

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"pault.ag/go/debian/dependency"
	"pault.ag/go/debian/version"
)

func decodeCustomValues(incoming reflect.Value, incomingField reflect.StructField, data string) error {
	/* Incoming is a slice */
	underlyingType := incoming.Type().Elem()

	var delim = " "
	if it := incomingField.Tag.Get("delim"); it != "" {
		delim = it
	}

	/* XXX: Fix stuff like []dependency.Dependency, since it's really really
	 *      silly. Perhaps we need some sort of function registration
	 *      magic. */

	for _, el := range strings.Split(data, delim) {
		targetValue := reflect.New(underlyingType)
		err := decodeValue(targetValue.Elem(), incomingField, el)
		if err != nil {
			return err
		}
		incoming.Set(reflect.Append(incoming, targetValue.Elem()))
	}
	return nil
}

func decodeCustomValue(incoming reflect.Value, incomingField reflect.StructField, data string) error {
	/* Check out the type */
	switch incoming.Type() {
	case reflect.TypeOf(dependency.Dependency{}):
		// {{{ pault.ag/go/debian/dependency.Dependency
		value, err := dependency.Parse(data)
		if err != nil {
			return err
		}
		incoming.Set(reflect.ValueOf(*value))
		return nil
		// }}}
	case reflect.TypeOf(version.Version{}):
		// {{{ pault.ag/go/debian/version.Version
		value, err := version.Parse(data)
		if err != nil {
			return err
		}
		incoming.Set(reflect.ValueOf(value))
		return nil
		// }}}
	case reflect.TypeOf(dependency.Arch{}):
		// {{{ pault.ag/go/debian/dependency.Arch
		value, err := dependency.ParseArch(data)
		if err != nil {
			return err
		}
		incoming.Set(reflect.ValueOf(*value))
		return nil
		// }}}
	}
	return fmt.Errorf("Unknown custom field type: %s", incoming.Type())
}

func decodeValue(incoming reflect.Value, incomingField reflect.StructField, data string) error {
	switch incoming.Type().Kind() {
	case reflect.String:
		incoming.SetString(data)
		return nil
	case reflect.Int:
		if data == "" {
			incoming.SetInt(0)
			return nil
		}
		value, err := strconv.Atoi(data)
		if err != nil {
			return err
		}
		incoming.SetInt(int64(value))
		return nil
	case reflect.Slice:
		return decodeCustomValues(incoming, incomingField, data)
	case reflect.Struct:
		return decodeCustomValue(incoming, incomingField, data)
	}
	return fmt.Errorf("Unknown type of field: %s", incoming.Type())
}

func decodePointer(incoming reflect.Value, data Paragraph) error {
	if incoming.Type().Kind() == reflect.Ptr {
		/* If we have a pointer, let's follow it */
		return decodePointer(incoming.Elem(), data)
	}

	for i := 0; i < incoming.NumField(); i++ {
		field := incoming.Field(i)
		fieldType := incoming.Type().Field(i)

		if field.Type().Kind() == reflect.Struct {
			err := decodePointer(field, data)
			if err != nil {
				return err
			}
		}

		paragraphKey := fieldType.Name
		if it := fieldType.Tag.Get("control"); it != "" {
			paragraphKey = it
		}

		if paragraphKey == "-" {
			continue
		}

		required := fieldType.Tag.Get("required") == "true"

		if val, ok := data.Values[paragraphKey]; ok {
			err := decodeValue(field, fieldType, val)
			if err != nil {
				return fmt.Errorf(
					"pault.ag/go/debian/control: failed to set %s: %s",
					fieldType.Name,
					err,
				)
			}
		} else if required {
			return fmt.Errorf(
				"pault.ag/go/debian/control: required field %s missing",
				fieldType.Name,
			)

		}
	}

	return nil
}

func Unmarshal(incoming interface{}, data io.Reader) error {
	reader := bufio.NewReader(data)
	para, err := ParseParagraph(reader)
	if err != nil {
		return err
	}
	return decodePointer(reflect.ValueOf(incoming), *para)
}

// vim: foldmethod=marker
