package internal

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

func PrintSplitFunc(word, field string) {
	for i, n := range strings.Split(word, ",") {
		if i == 0 {
			PrintFunc(field, n)
		} else {
			fmt.Printf("\t\t%s\n", n)
		}
	}
}

func PrintFunc(field, value string) {
	if len(field) < 8 {
		fmt.Printf("%s\t\t%s\n", color.HiBlackString(field), value)
	} else {
		fmt.Printf("%s\t%s\n", color.HiBlackString(field), value)
	}
}
