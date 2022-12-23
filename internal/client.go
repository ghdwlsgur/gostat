package internal

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// Split long titles to fit output format.
func PrintSplitFunc(word, field string) {
	for i, n := range strings.Split(word, ",") {
		if i == 0 {
			PrintFunc(field, n)
		} else {
			fmt.Printf("\t\t%s\n", n)
		}
	}
}

// Title text is displayed in black letters.
func PrintFunc(field, value string) {
	if len(field) < 8 {
		fmt.Printf("%s\t\t%s\n", color.HiBlackString(field), value)
	} else {
		fmt.Printf("%s\t%s\n", color.HiBlackString(field), value)
	}
}

func printStatusFormat(field string, valueA string, valueB string) {

	fieldLength := len(field) - 9

	if fieldLength < 8 {
		if len(valueA) <= 8 {
			fmt.Printf("\t%s\t\t\t%v\t\t\t\t%s\n", field, valueA, valueB)
			return
		}
		if len(valueA) <= 16 {
			fmt.Printf("\t%s\t\t\t%v\t\t\t%s\n", field, valueA, valueB)
			return
		}
		if len(valueA) <= 24 {
			fmt.Printf("\t%s\t\t\t%v\t\t%s\n", field, valueA, valueB)
			return
		}
	}

	if fieldLength < 16 {
		if len(valueA) <= 8 {
			fmt.Printf("\t%s\t%v\t\t\t\t%s\n", field, valueA, valueB)
			return
		}
		if len(valueA) <= 16 {
			fmt.Printf("\t%s\t\t%v\t\t\t%s\n", field, valueA, valueB)
			return
		}
		if len(valueA) <= 24 {
			fmt.Printf("\t%s\t\t%v\t\t%s\n", field, valueA, valueB)
			return
		}
	}

	if fieldLength < 24 {
		if len(valueA) <= 8 {
			fmt.Printf("\t%s\t%v\t\t\t\t%s\n", field, valueA, valueB)
			return
		}
		if len(valueA) <= 16 {
			fmt.Printf("\t%s\t%v\t\t\t%s\n", field, valueA, valueB)
			return
		}
		if len(valueA) <= 24 {
			fmt.Printf("\t%s\t%v\t\t%s\n", field, valueA, valueB)
			return
		}
	}

}

func stringFormat(word string) string {

	var prefixBucket []string
	words := strings.Split(word, "-")
	for i, w := range words {
		if i != len(words)-1 {
			prefixBucket = append(prefixBucket, w[:1])
		}
	}
	front := strings.Join(prefixBucket, "")
	wordFormat := strings.Join([]string{front, words[len(words)-1]}, "-")

	if len(wordFormat) > 14 {
		stringFormat(wordFormat)
	}
	return wordFormat
}

func printStatusToColor(status string) {

	stat := status[0:1]
	if stat == "5" {
		PrintFunc("Status", color.HiRedString(status))
	} else if stat == "4" {
		PrintFunc("Status", color.HiYellowString(status))
	} else {
		PrintFunc("Status", color.HiGreenString(status))
	}
}
