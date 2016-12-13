// Copyright 2016 Zack Guo <zack.y.guo@gmail.com>. All rights reserved.
// Use of this source code is governed by a MIT license that can
// be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	ui "github.com/gizak/termui"
)

// console embeds a ui.Par which is used to write out
// log messages. It keeps track of the messages in a
// buffer such that old messages can be evicted if the
// buffer is too full.
type console struct {
	*ui.Par

	msgs []string
}

// newConsole returns a new console
func newConsole(height int) *console {
	par := ui.NewPar("")
	par.Height = height
	par.BorderLabel = "Console"

	return &console{Par: par}
}

func (c *console) writeln(msg ...interface{}) {
	if len(c.msgs) > c.Par.Height-3 {
		c.msgs = c.msgs[1:]
	}
	c.msgs = append(c.msgs, fmt.Sprint(msg...))
	c.Par.Text = strings.Join(c.msgs, "\n")
}

func (c *console) writef(format string, a ...interface{}) {
	if len(c.msgs) > c.Par.Height-3 {
		c.msgs = c.msgs[1:]
	}
	c.msgs = append(c.msgs, fmt.Sprintf(format, a...))
	c.Par.Text = strings.Join(c.msgs, "\n")
}

func run(path string, console *console, gasGraph, blockTimeGraph *ui.Sparklines) error {
	client, err := ethclient.Dial(path)
	if err != nil {
		panic(err)
	}
	console.writeln("OK: Attached to client")

	var (
		ctx     = context.Background()
		million = big.NewInt(1000000)

		gasLimit  []int
		gasUsed   []int
		blockTime []int

		lastHeader *types.Header

		ch = make(chan *types.Header)
	)
	sub, err := client.SubscribeNewHead(ctx, ch)
	for {
		select {
		case header := <-ch:
			if len(gasLimit) == 100 {
				gasLimit = gasLimit[1:]
			}

			gasLimit = append(gasLimit, int(header.GasLimit.Div(header.GasLimit, million).Uint64()))
			gasGraph.Lines[0].Data = gasLimit

			if len(gasUsed) == 100 {
				gasUsed = gasUsed[1:]
			}

			gasUsed = append(gasUsed, int(header.GasUsed.Div(header.GasUsed, big.NewInt(100)).Uint64()))
			gasGraph.Lines[1].Data = gasUsed

			if lastHeader != nil {
				time := new(big.Int).Sub(header.Time, lastHeader.Time)
				blockTime = append(blockTime, int(time.Uint64()))
			}
			blockTimeGraph.Lines[0].Data = blockTime

			hash := header.Hash()
			console.writef("Added block: %d %x", header.Number, hash[:4])

			lastHeader = header
		case err := <-sub.Err():
			panic(err)
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage: %s /path/to/socket\n", os.Args[0])
		os.Exit(1)
	}

	if err := ui.Init(); err != nil {
		panic(err)
	}
	defer ui.Close()

	fmt.Println("initialising...")

	sp := newGasGraph()
	bt := newBlockTimeGraph()

	console := newConsole(7)

	// build layout
	ui.Body.AddRows(
		ui.NewRow(
			ui.NewCol(6, 0, sp),
			ui.NewCol(6, 0, bt),
		),
		ui.NewRow(ui.NewCol(12, 0, console)),
	)

	go run(os.Args[1], console, sp, bt)

	handleEvents()

	ui.Loop()
}

func handleEvents() {
	// calculate layout
	ui.Body.Align()

	ui.Handle("/sys/kbd/q", func(ui.Event) {
		ui.StopLoop()
	})
	ui.Handle("/timer/1s", func(e ui.Event) {
		ui.Render(ui.Body)
	})

	ui.Handle("/sys/wnd/resize", func(e ui.Event) {
		ui.Body.Width = ui.TermWidth()
		ui.Body.Align()
		ui.Clear()
		ui.Render(ui.Body)
	})
}

func newGasGraph() *ui.Sparklines {
	spark := ui.Sparkline{}
	spark.Height = 8
	spark.Title = "Gas limit"
	spark.LineColor = ui.ColorCyan
	spark.TitleColor = ui.ColorWhite

	spark2 := ui.Sparkline{}
	spark2.Height = 8
	spark2.Title = "Gas used"
	spark2.LineColor = ui.ColorRed
	spark2.TitleColor = ui.ColorWhite

	sp := ui.NewSparklines(spark, spark2)
	sp.Height = 20
	sp.BorderLabel = "Gas statistics"

	return sp
}

func newBlockTimeGraph() *ui.Sparklines {
	spark := ui.Sparkline{}
	spark.Height = 5
	spark.LineColor = ui.ColorMagenta
	spark.TitleColor = ui.ColorWhite

	sp := ui.NewSparklines(spark)
	sp.Height = 8
	sp.BorderLabel = "Block time"

	return sp
}
