package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

func FlagSetToArgs(fs *pflag.FlagSet, includeName bool) []string {
	var args []string
	if includeName {
		args = append(args, fs.Name())
	}
	fs.VisitAll(func(f *pflag.Flag) {
		valueStr := f.Value.String()
		if valueStr == f.DefValue {
			return
		}
		typ := f.Value.Type()
		switch typ {
		case "bool":
			if valueStr == "true" {
				args = append(args, fmt.Sprintf("--%s", f.Name))
			} else {
				args = append(args, fmt.Sprintf("--%s=false", f.Name))
			}
		case "string", "int", "ip", "bytesHex", "bytesBase64":
			args = append(args, fmt.Sprintf("--%s=%s", f.Name, valueStr))
		case "stringSlice", "stringArray":
			val := strings.TrimPrefix(valueStr, "[")
			val = strings.TrimSuffix(val, "]")
			if val == "" {
				return
			}
			elements := strings.Split(val, ",")
			for _, elem := range elements {
				args = append(args, fmt.Sprintf("--%s=%s", f.Name, elem))
			}
		default:
			if strings.HasSuffix(typ, "Slice") || strings.HasSuffix(typ, "Array") {
				val := strings.TrimPrefix(valueStr, "[")
				val = strings.TrimSuffix(val, "]")
				if val != "" {
					elements := strings.Split(val, ",")
					for _, elem := range elements {
						args = append(args, fmt.Sprintf("--%s=%s", f.Name, elem))
					}
				}
			} else {
				args = append(args, fmt.Sprintf("--%s=%s", f.Name, valueStr))
			}
		}
	})
	args = append(args, fs.Args()...)
	return args
}
