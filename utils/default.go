package utils

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/fatih/structs"
)

var (
	bytes     = []byte{8, 27, 91, 75}
	bytesLine = []byte{27, 91, 63, 49, 48, 51, 52, 104}
	ctrlL     = []byte{27, 91, 72, 27, 91, 50, 74, 98, 97, 115, 104, 45, 51, 46, 50, 36, 32}
	clear     = []byte{27, 91, 72, 27, 91, 50, 74}
	nr        = []byte{13, 10}
)

func ApplyDefaultValues(struct_ interface{}) (err error) {
	o := structs.New(struct_)

	for _, field := range o.Fields() {
		defaultValue := field.Tag("default")
		if defaultValue == "" {
			continue
		}
		var val interface{}
		switch field.Kind() {
		case reflect.String:
			val = defaultValue
		case reflect.Bool:
			if defaultValue == "true" {
				val = true
			} else if defaultValue == "false" {
				val = false
			} else {
				return fmt.Errorf("invalid bool expression: %v, use true/false", defaultValue)
			}
		case reflect.Int:
			val, err = strconv.Atoi(defaultValue)
			if err != nil {
				return err
			}
		default:
			val = field.Value()
		}
		field.Set(val)
	}
	return nil
}

// EqualTwoSliceByBack Check whether the user presses the delete key
func EqualTwoSliceByBack(data []byte) bool {
	equal := true
	if len(data) != 4 {
		return false
	}

	for i := 0; i < len(data); i++ {
		if data[i] != bytes[i] {
			equal = false
			break
		}
	}
	return equal
}

func EqualTwoSliceByLine(data []byte) bool {
	equal := true
	if len(data) != 8 {
		return false
	}

	for i := 0; i < len(data); i++ {
		if data[i] != bytesLine[i] {
			equal = false
			break
		}
	}
	return equal
}

// DeleteBel Check whether the user presses tab
func DeleteBel(data []byte) (result []byte) {
	for _, v := range data {
		if v != uint8(7) {
			result = append(result, v)
		}
	}
	return
}

// DeleteBs Check whether the user presses bs
func DeleteBs(data []byte) (result []byte) {
	for _, v := range data {
		if v != uint8(8) {
			result = append(result, v)
		}
	}
	return
}

// EqualTwoSliceByCtrlL Check whether the user presses CTRL +L
func EqualTwoSliceByCtrlL(data []byte) bool {
	equal := true
	if len(data) != 17 {
		return false
	}

	for i := 0; i < len(data); i++ {
		if data[i] != ctrlL[i] {
			equal = false
			break
		}
	}
	return equal
}

// EqualTwoSliceByClear Check whether the user presses clear
func EqualTwoSliceByClear(data []byte) bool {
	equal := true
	if len(data) != 7 {
		return false
	}

	for i := 0; i < len(data); i++ {
		if data[i] != clear[i] {
			equal = false
			break
		}
	}
	return equal
}

// EqualNR Check whether it is \r\n
func EqualNR(data []byte) bool {
	equal := true
	if len(data) != 2 {
		return false
	}

	for i := 0; i < len(data); i++ {
		if data[i] != nr[i] {
			equal = false
			break
		}
	}
	return equal
}

func FilterOutput(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if EqualTwoSliceByLine(data) || EqualTwoSliceByCtrlL(data) || EqualTwoSliceByClear(data) {
		return true
	}
	return false
}

// ExistBytes judge the head whether or not exist [27,91,67]
func ExistBytes(data []byte) bool {
	if len(data) > 4 && data[0] == uint8(27) && data[1] == uint8(91) && data[2] == uint8(67) {
		return true
	}
	if len(data) == 3 && data[0] == uint8(27) && data[1] == uint8(91) && data[2] == uint8(67) {
		return true
	}
	if len(data) == 4 && data[0] == uint8(27) && data[1] == uint8(91) && data[2] == uint8(49) && data[3] == uint8(80) {
		return true
	}
	return false
}

//ExistVi judge the string  exist vi/vim/sudo vi/sudo vim
func ExistVi(s string) bool {
	if strings.HasPrefix(strings.ToLower(s), "vi") || strings.HasPrefix(strings.ToLower(s), "vim") ||
		strings.HasPrefix(strings.ToLower(s), "sudo vi") || strings.HasPrefix(strings.ToLower(s), "sudo vim") {
		return true
	}
	return false
}
