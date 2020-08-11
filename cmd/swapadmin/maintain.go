package main

import (
	"fmt"

	"github.com/anyswap/CrossChain-Bridge/cmd/utils"
	"github.com/anyswap/CrossChain-Bridge/log"
	"github.com/urfave/cli/v2"
)

var (
	maintainCommand = &cli.Command{
		Action:    maintain,
		Name:      "maintain",
		Usage:     "admin maintain",
		ArgsUsage: "<open|close> <deposit|withdraw|both>",
		Description: `
admin maintain, open or close deposit and withdraw
`,
		Flags: commonAdminFlags,
	}
)

func maintain(ctx *cli.Context) error {
	utils.SetLogger(ctx)
	method := "maintain"
	if ctx.NArg() != 2 {
		_ = cli.ShowCommandHelp(ctx, method)
		fmt.Println()
		return fmt.Errorf("invalid arguments: %q", ctx.Args())
	}

	err := prepare(ctx)
	if err != nil {
		return err
	}

	operation := ctx.Args().Get(0)
	direction := ctx.Args().Get(1)

	switch operation {
	case "open", "close":
	default:
		return fmt.Errorf("unknown operation '%v'", operation)
	}

	switch direction {
	case "deposit", "withdraw", "both":
	default:
		return fmt.Errorf("unknown direction '%v'", direction)
	}

	log.Printf("admin maintain: %v %v", operation, direction)

	params := []string{operation, direction}
	result, err := adminCall(method, params)

	log.Printf("result is '%v'", result)
	return err
}
